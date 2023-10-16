package subcmd

import (
	"os"
	"path/filepath"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bputCmd = &cobra.Command{
	Use:     "bput [local file1] [local file2] [local dir1] ... [collection]",
	Aliases: []string{"bundle_put"},
	Short:   "Bundle-upload files or directories",
	Long:    `This uploads files or directories to the given iRODS collection. The files or directories are bundled with TAR to maximize data transfer bandwidth, then extracted in the iRODS.`,
	RunE:    processBputCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bputCmd)

	flag.SetBundleTempFlags(bputCmd)
	flag.SetBundleClearFlags(bputCmd)
	flag.SetBundleConfigFlags(bputCmd)
	flag.SetParallelTransferFlags(bputCmd, true)
	flag.SetForceFlags(bputCmd, true)
	flag.SetProgressFlags(bputCmd)
	flag.SetRetryFlags(bputCmd)
	flag.SetDifferentialTransferFlags(bputCmd, true)

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBputCommand",
	})

	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	bundleTempFlagValues := flag.GetBundleTempFlagValues()
	bundleClearFlagValues := flag.GetBundleClearFlagValues()
	bundleConfigFlagValues := flag.GetBundleConfigFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()
	differentialTransferFlagValues := flag.GetDifferentialTransferFlagValues()

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 + 2 // 2 for metadata op, 2 for extraction

	// clear local
	if bundleClearFlagValues.Clear {
		commons.CleanUpOldLocalBundles(bundleTempFlagValues.LocalTempPath, true)
	}

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := "./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	_, err = commons.StatIRODSPath(filesystem, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
	}

	logger.Info("determining staging dir...")
	if len(bundleTempFlagValues.IRODSTempPath) > 0 {
		logger.Debugf("validating staging dir - %s", bundleTempFlagValues.IRODSTempPath)

		bundleTempFlagValues.IRODSTempPath = commons.MakeIRODSPath(cwd, home, zone, bundleTempFlagValues.IRODSTempPath)
		ok, err := commons.ValidateStagingDir(filesystem, targetPath, bundleTempFlagValues.IRODSTempPath)
		if err != nil {
			return xerrors.Errorf("failed to validate staging dir - %s: %w", bundleTempFlagValues.IRODSTempPath, err)
		}

		if !ok {
			logger.Debugf("unable to use the given staging dir %s since it is in a different resource server, using default staging dir", bundleTempFlagValues.IRODSTempPath)
			return xerrors.Errorf("staging dir %s is in a different resource server", bundleTempFlagValues.IRODSTempPath)
		}
	} else {
		// set default staging dir
		logger.Debug("get default staging dir")

		bundleTempFlagValues.IRODSTempPath = commons.GetDefaultStagingDir(targetPath)
	}

	err = commons.CheckSafeStagingDir(bundleTempFlagValues.IRODSTempPath)
	if err != nil {
		return xerrors.Errorf("failed to get safe staging dir: %w", err)
	}

	logger.Infof("use staging dir - %s", bundleTempFlagValues.IRODSTempPath)

	if bundleClearFlagValues.Clear {
		logger.Debugf("clearing irods temp dir %s", bundleTempFlagValues.IRODSTempPath)
		commons.CleanUpOldIRODSBundles(filesystem, bundleTempFlagValues.IRODSTempPath, false, true)
	}

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, bundleConfigFlagValues.MaxFileNum, bundleConfigFlagValues.MaxFileSize, parallelTransferFlagValues.SingleTread, parallelTransferFlagValues.ThreadNumber, bundleTempFlagValues.LocalTempPath, bundleTempFlagValues.IRODSTempPath, differentialTransferFlagValues.DifferentialTransfer, differentialTransferFlagValues.NoHash, bundleConfigFlagValues.NoBulkRegistration, progressFlagValues.ShowProgress)
	bundleTransferManager.Start()

	bundleRootPath, err := commons.GetCommonRootLocalDirPath(sourcePaths)
	if err != nil {
		return xerrors.Errorf("failed to get common root dir for source paths: %w", err)
	}

	bundleTransferManager.SetBundleRootPath(bundleRootPath)

	for _, sourcePath := range sourcePaths {
		err = bputOne(bundleTransferManager, sourcePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to perform bput %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	bundleTransferManager.DoneScheduling()
	err = bundleTransferManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform bundle transfer: %w", err)
	}

	return nil
}

func bputOne(bundleManager *commons.BundleTransferManager, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "bputOne",
	})

	sourcePath = commons.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		bundleManager.Schedule(sourcePath, sourceStat.Size(), sourceStat.ModTime().Local())
	} else {
		// dir
		logger.Debugf("bundle-uploading a local directory %s", sourcePath)

		walkFunc := func(path string, entry os.DirEntry, err2 error) error {
			if err2 != nil {
				return xerrors.Errorf("failed to walk for %s: %w", path, err2)
			}

			if entry.IsDir() {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return xerrors.Errorf("failed to get info for %s: %w", path, err)
			}

			err = bundleManager.Schedule(path, info.Size(), info.ModTime())
			if err != nil {
				return xerrors.Errorf("failed to schedule %s: %w", path, err)
			}
			return nil
		}

		err := filepath.WalkDir(sourcePath, walkFunc)
		if err != nil {
			return xerrors.Errorf("failed to walk for %s: %w", sourcePath, err)
		}
	}
	return nil
}

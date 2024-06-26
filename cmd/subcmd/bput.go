package subcmd

import (
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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
	flag.SetCommonFlags(bputCmd, false)

	flag.SetBundleTempFlags(bputCmd)
	flag.SetBundleClearFlags(bputCmd)
	flag.SetBundleConfigFlags(bputCmd)
	flag.SetParallelTransferFlags(bputCmd, true)
	flag.SetForceFlags(bputCmd, true)
	flag.SetProgressFlags(bputCmd)
	flag.SetRetryFlags(bputCmd)
	flag.SetDifferentialTransferFlags(bputCmd, true)
	flag.SetNoRootFlags(bputCmd)
	flag.SetSyncFlags(bputCmd)

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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
	noRootFlagValues := flag.GetNoRootFlagValues()
	syncFlagValues := flag.GetSyncFlagValues()

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

	if noRootFlagValues.NoRoot && len(sourcePaths) > 1 {
		return xerrors.Errorf("failed to bput multiple source dirs without creating root directory")
	}

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	_, err = filesystem.StatDir(targetPath)
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

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, bundleConfigFlagValues.MaxFileNum, bundleConfigFlagValues.MaxFileSize, parallelTransferFlagValues.SingleTread, parallelTransferFlagValues.ThreadNumber, parallelTransferFlagValues.RedirectToResource, parallelTransferFlagValues.Icat, bundleTempFlagValues.LocalTempPath, bundleTempFlagValues.IRODSTempPath, differentialTransferFlagValues.DifferentialTransfer, differentialTransferFlagValues.NoHash, bundleConfigFlagValues.NoBulkRegistration, progressFlagValues.ShowProgress, progressFlagValues.ShowFullPath)
	bundleTransferManager.Start()

	if noRootFlagValues.NoRoot && len(sourcePaths) == 1 {
		bundleRootPath, err := commons.GetCommonRootLocalDirPathForSync(sourcePaths)
		if err != nil {
			return xerrors.Errorf("failed to get common root dir for source paths: %w", err)
		}

		bundleTransferManager.SetBundleRootPath(bundleRootPath)
	} else {
		bundleRootPath, err := commons.GetCommonRootLocalDirPath(sourcePaths)
		if err != nil {
			return xerrors.Errorf("failed to get common root dir for source paths: %w", err)
		}

		bundleTransferManager.SetBundleRootPath(bundleRootPath)
	}

	for _, sourcePath := range sourcePaths {
		err = bputOne(bundleTransferManager, sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to perform bput %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	bundleTransferManager.DoneScheduling()
	err = bundleTransferManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform bundle transfer: %w", err)
	}

	// delete extra
	if syncFlagValues.Delete {
		logger.Infof("deleting extra files and dirs under %s", targetPath)

		err = bputDeleteExtra(bundleTransferManager, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func bputOne(bundleManager *commons.BundleTransferManager, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "bputOne",
	})

	sourcePath = commons.MakeLocalPath(sourcePath)

	realSourcePath, err := commons.ResolveSymlink(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to resolve symlink %s: %w", sourcePath, err)
	}

	logger.Debugf("path %s ==> %s", sourcePath, realSourcePath)

	sourceStat, err := os.Stat(realSourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(realSourcePath)
		}

		return xerrors.Errorf("failed to stat %s: %w", realSourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		err = bundleManager.Schedule(sourcePath, false, sourceStat.Size(), sourceStat.ModTime().Local())
		if err != nil {
			return xerrors.Errorf("failed to schedule %s: %w", sourcePath, err)
		}
	} else {
		// dir
		logger.Debugf("bundle-uploading a local directory %s", sourcePath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		for _, entry := range entries {
			entryPath := filepath.Join(sourcePath, entry.Name())
			err = bputOne(bundleManager, entryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func bputDeleteExtra(bundleManager *commons.BundleTransferManager, targetPath string) error {
	pathMap := bundleManager.GetInputPathMap()
	filesystem := bundleManager.GetFilesystem()

	return bputDeleteExtraInternal(filesystem, pathMap, targetPath)
}

func bputDeleteExtraInternal(filesystem *irodsclient_fs.FileSystem, inputPathMap map[string]bool, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "bputDeleteExtraInternal",
	})

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra data object %s", targetPath)
			removeErr := filesystem.RemoveFile(targetPath, true)
			if removeErr != nil {
				return removeErr
			}
		}
	} else {
		// dir
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra dir
			logger.Debugf("removing an extra collection %s", targetPath)
			removeErr := filesystem.RemoveDir(targetPath, true, true)
			if removeErr != nil {
				return removeErr
			}
		} else {
			// non extra dir
			entries, err := filesystem.List(targetPath)
			if err != nil {
				return xerrors.Errorf("failed to list dir %s: %w", targetPath, err)
			}

			for idx := range entries {
				newTargetPath := entries[idx].Path

				err = bputDeleteExtraInternal(filesystem, inputPathMap, newTargetPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

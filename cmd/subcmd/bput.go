package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var bputCmd = &cobra.Command{
	Use:   "bput [local file1] [local file2] [local dir1] ... [collection]",
	Short: "Bundle-upload files or directories",
	Long:  `This uploads files or directories to the given iRODS collection. The files or directories are bundled with TAR to maximize data transfer bandwidth, then extracted in the iRODS.`,
	RunE:  processBputCommand,
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(bputCmd)

	bputCmd.Flags().Int("max_file_num", commons.MaxBundleFileNumDefault, "Specify max file number in a bundle file")
	bputCmd.Flags().Int64("max_file_size", commons.MaxBundleFileSizeDefault, "Specify max file size of a bundle file")
	bputCmd.Flags().Bool("progress", false, "Display progress bars")
	bputCmd.Flags().String("local_temp", os.TempDir(), "Specify local temp directory path to create bundle files")
	bputCmd.Flags().String("irods_temp", "", "Specify iRODS temp directory path to upload bundle files to")
	bputCmd.Flags().Bool("diff", false, "Put files having different content")
	bputCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBputCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	maxFileNum := commons.MaxBundleFileNumDefault
	maxFileNumFlag := command.Flags().Lookup("max_file_num")
	if maxFileNumFlag != nil {
		n, err := strconv.ParseInt(maxFileNumFlag.Value.String(), 10, 32)
		if err == nil {
			maxFileNum = int(n)
		}
	}

	maxFileSize := commons.MaxBundleFileSizeDefault
	maxFileSizeFlag := command.Flags().Lookup("max_file_size")
	if maxFileSizeFlag != nil {
		n, err := strconv.ParseInt(maxFileSizeFlag.Value.String(), 10, 64)
		if err == nil {
			maxFileSize = n
		}
	}

	progress := false
	progressFlag := command.Flags().Lookup("progress")
	if progressFlag != nil {
		progress, err = strconv.ParseBool(progressFlag.Value.String())
		if err != nil {
			progress = false
		}
	}

	diff := false
	diffFlag := command.Flags().Lookup("diff")
	if diffFlag != nil {
		diff, err = strconv.ParseBool(diffFlag.Value.String())
		if err != nil {
			diff = false
		}
	}

	noHash := false
	noHashFlag := command.Flags().Lookup("no_hash")
	if noHashFlag != nil {
		noHash, err = strconv.ParseBool(noHashFlag.Value.String())
		if err != nil {
			noHash = false
		}
	}

	localTempDirPath := os.TempDir()
	localTempPathFlag := command.Flags().Lookup("local_temp")
	if localTempPathFlag != nil {
		localTempDirPath = localTempPathFlag.Value.String()
	}

	irodsTempDirPath := ""
	irodsTempPathFlag := command.Flags().Lookup("irods_temp")
	if irodsTempPathFlag != nil {
		tempDirPath := irodsTempPathFlag.Value.String()
		if len(tempDirPath) > 0 {
			irodsTempDirPath = tempDirPath
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	if len(args) == 0 {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

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
	if len(irodsTempDirPath) > 0 {
		logger.Debugf("validating staging dir - %s", irodsTempDirPath)

		irodsTempDirPath = commons.MakeIRODSPath(cwd, home, zone, irodsTempDirPath)
		ok, err := commons.ValidateStagingDir(filesystem, targetPath, irodsTempDirPath)
		if err != nil {
			logger.WithError(err).Errorf("failed to validate staging dir - %s", irodsTempDirPath)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}

		if !ok {
			logger.Debugf("unable to use the given staging dir %s since it is in a different resource server, using default staging dir", irodsTempDirPath)

			irodsTempDirPath = commons.GetDefaultStagingDirInTargetPath(targetPath)
		}
	}

	if len(irodsTempDirPath) == 0 {
		// set default staging dir
		logger.Debug("get default staging dir")

		irodsTempDirPath, err = commons.GetDefaultStagingDir(filesystem, targetPath)
		if err != nil {
			logger.WithError(err).Error("failed to get default staging dir")
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	logger.Debugf("use staging dir - %s", irodsTempDirPath)

	// clean up staging dir in the target dir
	defer func() {
		unusedStagingDir := commons.GetDefaultStagingDirInTargetPath(targetPath)
		logger.Debugf("delete staging dir - %s", unusedStagingDir)
		err := filesystem.RemoveDir(unusedStagingDir, true, true)
		if err != nil {
			logger.WithError(err).Warnf("failed to delete staging dir - %s, remove it manually later", unusedStagingDir)
		}
	}()

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, maxFileNum, maxFileSize, localTempDirPath, irodsTempDirPath, diff, noHash, progress)
	bundleTransferManager.Start()

	bundleRootPath, err := commons.GetCommonRootLocalDirPath(sourcePaths)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	bundleTransferManager.SetBundleRootPath(bundleRootPath)

	for _, sourcePath := range sourcePaths {
		err = bputOne(bundleTransferManager, sourcePath, targetPath)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	bundleTransferManager.DoneScheduling()
	err = bundleTransferManager.Wait()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
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
		return err
	}

	if !sourceStat.IsDir() {
		bundleManager.Schedule(sourcePath, sourceStat.Size(), sourceStat.ModTime().Local())
	} else {
		// dir
		logger.Debugf("bundle-uploading a local directory %s", sourcePath)

		walkFunc := func(path string, entry os.DirEntry, err2 error) error {
			if err2 != nil {
				return err2
			}

			if entry.IsDir() {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			bundleManager.Schedule(path, info.Size(), info.ModTime())

			if err != nil {
				return err
			}
			return nil
		}

		err := filepath.WalkDir(sourcePath, walkFunc)
		if err != nil {
			return err
		}
	}
	return nil
}

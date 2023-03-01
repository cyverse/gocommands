package subcmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var syncCmd = &cobra.Command{
	Use:   "sync i:[collection] [local dir] or sync [local dir] i:[collection]",
	Short: "Sync local directory with iRODS collection",
	Long:  `This synchronizes a local directory with the given iRODS collection.`,
	RunE:  processSyncCommand,
}

func AddSyncCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(syncCmd)

	syncCmd.Flags().Int("max_file_num", commons.MaxBundleFileNumDefault, "Specify max file number in a bundle file")
	syncCmd.Flags().Int64("max_file_size", commons.MaxBundleFileSizeDefault, "Specify max file size of a bundle file")
	syncCmd.Flags().Int("upload_thread_num", commons.UploadTreadNumDefault, "Specify the number of upload threads")
	syncCmd.Flags().Bool("progress", false, "Display progress bar")
	syncCmd.Flags().String("local_temp", os.TempDir(), "Specify local temp directory path to create bundle files")
	syncCmd.Flags().String("irods_temp", "", "Specify iRODS temp directory path to upload bundle files to")
	syncCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")
	syncCmd.Flags().Bool("no_replication", false, "Disable replication (default is False)")
	syncCmd.Flags().Int("retry", 1, "Retry if fails (default is 1)")
	syncCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds (default is 60)")

	rootCmd.AddCommand(syncCmd)
}

func processSyncCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
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

	uploadThreadNum := commons.UploadTreadNumDefault
	uploadThreadNumFlag := command.Flags().Lookup("upload_thread_num")
	if uploadThreadNumFlag != nil {
		n, err := strconv.ParseInt(uploadThreadNumFlag.Value.String(), 10, 32)
		if err == nil {
			uploadThreadNum = int(n)
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

	noHash := false
	noHashFlag := command.Flags().Lookup("no_hash")
	if noHashFlag != nil {
		noHash, err = strconv.ParseBool(noHashFlag.Value.String())
		if err != nil {
			noHash = false
		}
	}

	noReplication := false
	noReplicationFlag := command.Flags().Lookup("no_replication")
	if noReplicationFlag != nil {
		noReplication, err = strconv.ParseBool(noReplicationFlag.Value.String())
		if err != nil {
			noReplication = false
		}
	}

	replication := !noReplication

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

	retryChild := false
	retryChildFlag := command.Flags().Lookup("retry_child")
	if retryChildFlag != nil {
		retryChildValue, err := strconv.ParseBool(retryChildFlag.Value.String())
		if err != nil {
			retryChildValue = false
		}

		retryChild = retryChildValue
	}

	retry := int64(1)
	retryFlag := command.Flags().Lookup("retry")
	if retryFlag != nil {
		retry, err = strconv.ParseInt(retryFlag.Value.String(), 10, 32)
		if err != nil {
			retry = 1
		}
	}

	retryInterval := int64(60)
	retryIntervalFlag := command.Flags().Lookup("retry_interval")
	if retryIntervalFlag != nil {
		retryInterval, err = strconv.ParseInt(retryIntervalFlag.Value.String(), 10, 32)
		if err != nil {
			retryInterval = 60
		}
	}

	if retry > 1 && !retryChild {
		err = commons.RunWithRetry(int(retry), int(retryInterval))
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retry, err)
		}
		return nil
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(args) < 2 {
		return xerrors.Errorf("not enough input arguments")
	}

	targetPath := "i:./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	localSources := []string{}
	irodsSources := []string{}

	for _, sourcePath := range sourcePaths {
		if strings.HasPrefix(sourcePath, "i:") {
			irodsSources = append(irodsSources, sourcePath)
		} else {
			localSources = append(localSources, sourcePath)
		}
	}

	if len(localSources) > 0 {
		// source is local
		if !strings.HasPrefix(targetPath, "i:") {
			// local to local
			return xerrors.Errorf("syncing between local files/directories is not supported")
		}

		// target must starts with "i:"
		err := syncFromLocal(filesystem, localSources, targetPath[2:], maxFileNum, maxFileSize, uploadThreadNum, localTempDirPath, irodsTempDirPath, progress, replication, noHash)
		if err != nil {
			return xerrors.Errorf("failed to perform sync (from local): %w", err)
		}
	}

	if len(irodsSources) > 0 {
		// source is iRODS
		err := syncFromRemote(filesystem, irodsSources, targetPath, progress, noHash)
		if err != nil {
			return xerrors.Errorf("failed to perform sync (from remote): %w", err)
		}
	}

	return nil
}

func syncFromLocal(filesystem *fs.FileSystem, sourcePaths []string, targetPath string, maxFileNum int, maxFileSize int64, uploadThreadNum int, localTempDirPath string, irodsTempDirPath string, progress bool, replication bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncFromLocal",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	fmt.Printf("determining staging dir...\n")
	if len(irodsTempDirPath) > 0 {
		logger.Debugf("validating staging dir - %s", irodsTempDirPath)

		irodsTempDirPath = commons.MakeIRODSPath(cwd, home, zone, irodsTempDirPath)
		ok, err := commons.ValidateStagingDir(filesystem, targetPath, irodsTempDirPath)
		if err != nil {
			return xerrors.Errorf("failed to validate staging dir - %s: %w", irodsTempDirPath, err)
		}

		if !ok {
			logger.Debugf("unable to use the given staging dir %s since it is in a different resource server, using default staging dir", irodsTempDirPath)

			irodsTempDirPath = commons.GetDefaultStagingDirInTargetPath(targetPath)
		}
	}

	var err error
	if len(irodsTempDirPath) == 0 {
		// set default staging dir
		logger.Debug("get default staging dir")

		irodsTempDirPath, err = commons.GetDefaultStagingDir(filesystem, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get default staging dir: %w", err)
		}
	}

	logger.Debugf("use staging dir - %s", irodsTempDirPath)
	fmt.Printf("will use %s for staging\n", irodsTempDirPath)

	// clean up staging dir in the target dir
	defer func() {
		unusedStagingDir := commons.GetDefaultStagingDirInTargetPath(targetPath)
		logger.Debugf("delete staging dir - %s", unusedStagingDir)
		err := filesystem.RemoveDir(unusedStagingDir, true, true)
		if err != nil {
			logger.WithError(err).Errorf("failed to delete staging dir - %s, remove it manually later", unusedStagingDir)
		}
	}()

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, maxFileNum, maxFileSize, uploadThreadNum, localTempDirPath, irodsTempDirPath, true, noHash, replication, progress)
	bundleTransferManager.Start()

	bundleRootPath, err := commons.GetCommonRootLocalDirPathForSync(sourcePaths)
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

func syncFromRemote(filesystem *fs.FileSystem, sourcePaths []string, targetPath string, progress bool, noHash bool) error {
	parallelJobManager := commons.NewParallelJobManager(filesystem, commons.MaxThreadNumDefault, progress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		// sourcePath must starts with "i:"
		if strings.HasPrefix(targetPath, "i:") {
			// copy
			err := copyOne(parallelJobManager, sourcePath[2:], targetPath[2:], true, false, true, noHash)
			if err != nil {
				return xerrors.Errorf("failed to perform copy %s to %s: %w", sourcePath[2:], targetPath[2:], err)
			}
		} else {
			// get
			err := getOne(parallelJobManager, sourcePath[2:], targetPath, false, true, noHash)
			if err != nil {
				return xerrors.Errorf("failed to perform get %s to %s: %w", sourcePath[2:], targetPath, err)
			}
		}
	}

	parallelJobManager.DoneScheduling()
	err := parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	return nil
}

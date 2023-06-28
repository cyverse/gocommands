package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(bputCmd)

	// unused, but required for compatibility with retry
	bputCmd.Flags().BoolP("force", "f", false, "unused")
	bputCmd.Flags().MarkHidden("force")
	bputCmd.Flags().Bool("clear_leftover", false, "Clear leftover bundle files")
	bputCmd.Flags().Int("max_file_num", commons.MaxBundleFileNumDefault, "Specify max file number in a bundle file")
	bputCmd.Flags().String("max_file_size", strconv.FormatInt(commons.MaxBundleFileSizeDefault, 10), "Specify max file size of a bundle file")
	bputCmd.Flags().Bool("single_threaded", false, "Transfer a file using a single thread")
	bputCmd.Flags().Int("upload_thread_num", commons.UploadTreadNumDefault, "Specify the number of upload threads")
	bputCmd.Flags().String("tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	bputCmd.Flags().Bool("progress", false, "Display progress bars")
	bputCmd.Flags().String("local_temp", os.TempDir(), "Specify local temp directory path to create bundle files")
	bputCmd.Flags().String("irods_temp", "", "Specify iRODS temp directory path to upload bundle files to")
	bputCmd.Flags().Bool("diff", false, "Put files having different content")
	bputCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")
	bputCmd.Flags().Bool("no_replication", false, "Disable replication")
	bputCmd.Flags().Int("retry", 1, "Retry if fails")
	bputCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds")

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBputCommand",
	})

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

	clearLeftover := false
	clearLeftoverFlag := command.Flags().Lookup("clear_leftover")
	if clearLeftoverFlag != nil {
		clearLeftover, err = strconv.ParseBool(clearLeftoverFlag.Value.String())
		if err != nil {
			clearLeftover = false
		}
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
		n, err := commons.ParseSize(maxFileSizeFlag.Value.String())
		if err == nil {
			maxFileSize = n
		}
	}

	singleThreaded := false
	singleThreadedFlag := command.Flags().Lookup("single_threaded")
	if singleThreadedFlag != nil {
		singleThreaded, err = strconv.ParseBool(singleThreadedFlag.Value.String())
		if err != nil {
			singleThreaded = false
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

	maxConnectionNum := uploadThreadNum + 2 + 2 // 2 for metadata op, 2 for extraction

	tcpBufferSize := commons.TcpBufferSizeDefault
	tcpBufferSizeFlag := command.Flags().Lookup("tcp_buffer_size")
	if tcpBufferSizeFlag != nil {
		n, err := commons.ParseSize(tcpBufferSizeFlag.Value.String())
		if err == nil {
			tcpBufferSize = int(n)
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

	// clear local
	if clearLeftover {
		commons.CleanUpOldLocalBundles(localTempDirPath, true)
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, tcpBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	if clearLeftover {
		trashHome := commons.GetTrashHomeDir()
		logger.Debugf("clearing trash dir %s", trashHome)
		commons.CleanUpOldIRODSBundles(filesystem, trashHome, false, true)

		if len(irodsTempDirPath) > 0 {
			logger.Debugf("clearing irods temp dir %s", irodsTempDirPath)
			commons.CleanUpOldIRODSBundles(filesystem, irodsTempDirPath, false, true)
		}
	}

	if retry > 1 && !retryChild {
		// we release filesystem here to not hold idle connections
		filesystem.Release()

		err = commons.RunWithRetry(int(retry), int(retryInterval))
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retry, err)
		}
		return nil
	}

	// release filesystem after use
	defer filesystem.Release()

	if len(args) == 0 {
		return xerrors.Errorf("not enough input arguments")
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
		commons.CleanUpOldIRODSBundles(filesystem, unusedStagingDir, true, true)
	}()

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, maxFileNum, maxFileSize, singleThreaded, uploadThreadNum, localTempDirPath, irodsTempDirPath, diff, noHash, replication, progress)
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

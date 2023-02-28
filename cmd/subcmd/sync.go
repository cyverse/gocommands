package subcmd

import (
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

	syncCmd.Flags().Bool("progress", false, "Display progress bar")
	syncCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")
	syncCmd.Flags().Bool("no_replication", false, "Disable replication (default is False)")
	syncCmd.Flags().Int("retry", 1, "Retry if fails (default is 1)")

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

	if retry > 1 && !retryChild {
		err = commons.RunWithRetry(int(retry))
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", err)
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
		err := syncFromLocal(filesystem, localSources, targetPath[2:], progress, replication, noHash)
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

func syncFromLocal(filesystem *fs.FileSystem, sourcePaths []string, targetPath string, progress bool, replication bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncFromLocal",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	// set default staging dir
	logger.Debug("get default staging dir")

	irodsTempDirPath, err := commons.GetDefaultStagingDir(filesystem, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to get default staging dir: %w", err)
	}

	logger.Debugf("use staging dir - %s", irodsTempDirPath)

	// clean up staging dir in the target dir
	defer func() {
		unusedStagingDir := commons.GetDefaultStagingDirInTargetPath(targetPath)
		logger.Debugf("delete staging dir - %s", unusedStagingDir)
		err := filesystem.RemoveDir(unusedStagingDir, true, true)
		if err != nil {
			logger.WithError(err).Errorf("failed to delete staging dir - %s, remove it manually later", unusedStagingDir)
		}
	}()

	localTempDirPath := os.TempDir()

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, commons.MaxBundleFileNumDefault, commons.MaxBundleFileSizeDefault, localTempDirPath, irodsTempDirPath, true, noHash, replication, progress)
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

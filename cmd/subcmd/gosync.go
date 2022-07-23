package subcmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync i:[local dir] [collection] or sync [collection] i:[local dir]",
	Short: "Sync local directory with iRODS collection",
	Long:  `This synchronizes a local directory with the given iRODS collection.`,
	RunE:  processSyncCommand,
}

func AddSyncCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(syncCmd)

	syncCmd.Flags().BoolP("progress", "", false, "Display progress bar")

	rootCmd.AddCommand(syncCmd)
}

func processSyncCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSyncCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		logger.Error(err)
	}

	if !cont {
		return err
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		return err
	}

	progress := false
	progressFlag := command.Flags().Lookup("progress")
	if progressFlag != nil {
		progress, err = strconv.ParseBool(progressFlag.Value.String())
		if err != nil {
			progress = false
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	parallelTransferManager := commons.NewParallelTransferManager(commons.MaxThreadNum)

	if len(args) >= 2 {
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			if strings.HasPrefix(sourcePath, "i:") {
				if strings.HasPrefix(targetPath, "i:") {
					// copy
					err = syncCopyOne(parallelTransferManager, filesystem, sourcePath[2:], targetPath[2:])
					if err != nil {
						logger.Error(err)
						return err
					}
				} else {
					// get
					err = syncGetOne(parallelTransferManager, filesystem, sourcePath[2:], targetPath)
					if err != nil {
						logger.Error(err)
						return err
					}
				}
			} else {
				if strings.HasPrefix(targetPath, "i:") {
					// put
					err = syncPutOne(parallelTransferManager, filesystem, sourcePath, targetPath[2:])
					if err != nil {
						logger.Error(err)
						return err
					}
				} else {
					// local to local
					return fmt.Errorf("syncing between local files/directories is not supported")
				}
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}

	err = parallelTransferManager.Go(progress)
	if err != nil {
		logger.Error(err)
		return err
	}

	return nil
}

func syncGetOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncGetOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	entry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if entry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.EnsureTargetLocalFilePath(sourcePath, targetPath)

		logger.Debugf("scheduled synchronizing a data object %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleDownloadIfDifferent(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("synchronizing a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(entry.Path)
		if err != nil {
			return err
		}

		// make target dir if not exists
		err = os.MkdirAll(targetPath, 0766)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			targetEntryPath := filepath.Join(targetPath, entryInDir.Name)
			err = syncGetOne(transferManager, filesystem, entryInDir.Path, targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncPutOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncPutOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	st, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		targetFilePath := commons.EnsureTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		logger.Debugf("scheduled synchronizing a local file %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleUploadIfDifferent(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("synchronizing a local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}

		// make target dir if not exists
		if !filesystem.ExistsDir(targetPath) {
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return err
			}
		}

		for _, entryInDir := range entries {
			targetEntryPath := path.Join(targetPath, entryInDir.Name())
			err = syncPutOne(transferManager, filesystem, filepath.Join(sourcePath, entryInDir.Name()), targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncCopyOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncCopyOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.EnsureTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		logger.Debugf("scheduled synchronizing a data object %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleCopyIfDifferent(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("synchronizing a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return err
		}

		// make target dir if not exists
		if !filesystem.ExistsDir(targetPath) {
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return err
			}
		}

		for _, entryInDir := range entries {
			targetEntryPath := path.Join(targetPath, entryInDir.Name)
			err = syncCopyOne(transferManager, filesystem, entryInDir.Path, targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [data-object1] [data-object2] [collection1] ... [local dir]",
	Short: "Download iRODS data-objects or collections",
	Long:  `This downloads iRODS data-objects or collections to the given local path.`,
	RunE:  processGetCommand,
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(getCmd)

	getCmd.Flags().BoolP("force", "f", false, "Get forcefully")

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processGetCommand",
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
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

	if len(args) == 1 {
		// download to current dir
		err = getOne(parallelTransferManager, filesystem, args[0], "./", force)
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 2 {
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = getOne(parallelTransferManager, filesystem, sourcePath, targetPath, force)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}

	err = parallelTransferManager.Go()
	if err != nil {
		logger.Error(err)
		return err
	}

	return nil
}

func getOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.EnsureTargetLocalFilePath(sourcePath, targetPath)

		targetStat, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			// file/dir exists
			if targetStat.IsDir() {
				// dir
				return fmt.Errorf("local path %s is a directory", targetFilePath)
			}

			if force {
				// delete first
				logger.Debugf("deleting an existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing file %s", targetFilePath)
					err := os.Remove(targetFilePath)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		logger.Debugf("scheduled a data object download %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleDownload(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := filepath.Join(targetPath, sourceEntry.Name)
		err = os.MkdirAll(targetDir, 0766)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			err = getOne(transferManager, filesystem, entryInDir.Path, targetDir, force)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

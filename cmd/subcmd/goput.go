package subcmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:   "put [local file1] [local file2] [local dir1] ... [collection]",
	Short: "Upload files or directories",
	Long:  `This uploads files or directories to the given iRODS collection.`,
	RunE:  processPutCommand,
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(putCmd)

	putCmd.Flags().BoolP("force", "f", false, "Put forcefully")
	putCmd.Flags().BoolP("progress", "", false, "Display progress bar")

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processPutCommand",
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
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

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	parallelTransferManager := commons.NewParallelTransferManager(commons.MaxThreadNum)

	if len(args) == 1 {
		// upload to current collection
		err = putOne(parallelTransferManager, filesystem, args[0], "./", force)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	} else if len(args) >= 2 {
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = putOne(parallelTransferManager, filesystem, sourcePath, targetPath, force)
			if err != nil {
				logger.Error(err)
				fmt.Fprintln(os.Stderr, err.Error())
				return nil
			}
		}
	} else {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	err = parallelTransferManager.Go(progress)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func putOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !sourceStat.IsDir() {
		targetFilePath := commons.EnsureTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		if filesystem.ExistsFile(targetFilePath) {
			// already exists!
			if force {
				// delete first
				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s", targetFilePath)
					err := filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip uploading a file %s. The data object already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		logger.Debugf("scheduled a local file upload %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleUpload(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("uploading a local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := path.Join(targetPath, filepath.Base(sourcePath))
		err = filesystem.MakeDir(targetDir, true)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			err = putOne(transferManager, filesystem, filepath.Join(sourcePath, entryInDir.Name()), targetDir, force)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

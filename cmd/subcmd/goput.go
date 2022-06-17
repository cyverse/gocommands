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

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processPutCommand",
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

	if len(args) == 1 {
		// upload to current collection
		err = putOne(filesystem, args[0], "./", force)
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 2 {
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = putOne(filesystem, sourcePath, targetPath, force)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}
	return nil
}

func putOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putOne",
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

		if filesystem.ExistsFile(targetFilePath) {
			// already exists!
			if force {
				// delete first
				logger.Debugf("deleting an existing data object %s")
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s")
					err := filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip uploading a file %s. The file already exists!\n", targetFilePath)
					//return fmt.Errorf("file %s already exists, turn on 'force' option to overwrite", targetFilePath)
					return nil
				}
			}
		}

		logger.Debugf("uploading a local file %s to %s", sourcePath, targetFilePath)
		err := filesystem.UploadFileParallel(sourcePath, targetPath, "", 0, true)
		if err != nil {
			return err
		}
	} else {
		// dir
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := filepath.Join(targetPath, filepath.Base(sourcePath))
		err = filesystem.MakeDir(targetDir, true)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			err = putOne(filesystem, filepath.Join(sourcePath, entryInDir.Name()), targetDir, force)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

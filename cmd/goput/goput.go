package main

import (
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "goput [local file1] [local file2] [local dir1] ... [collection]",
	Short: "Upload files or directories",
	Long:  `This uploads files or directories to the given iRODS collection.`,
	RunE:  processCommand,
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
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

	// Create a file system
	account := commons.GetAccount()

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-put")
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) == 1 {
		// upload to current collection
		err = putOne(filesystem, args[0], "./")
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 2 {
		irodsPath := args[len(args)-1]
		for _, filePath := range args[:len(args)-1] {
			err = putOne(filesystem, filePath, irodsPath)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	}

	return nil
}

func main() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	commons.SetCommonFlags(rootCmd)

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

func putOne(filesystem *irodsclient_fs.FileSystem, filePath string, targetPath string) error {
	filePath = commons.MakeLocalPath(filePath)
	cwd := commons.GetCWD()
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	st, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return putDataObject(filesystem, filePath, targetPath)
	} else {
		// dir
		entries, err := os.ReadDir(filePath)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := filepath.Join(targetPath, filepath.Base(filePath))
		err = filesystem.MakeDir(targetDir, true)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			err = putOne(filesystem, filepath.Join(filePath, entryInDir.Name()), targetDir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func putDataObject(filesystem *irodsclient_fs.FileSystem, filePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putDataObject",
	})

	logger.Debugf("uploading a file %s to an iRODS collection %s\n", filePath, targetPath)

	err := filesystem.UploadFileParallel(filePath, targetPath, "", 0, false)
	if err != nil {
		return err
	}

	return nil
}

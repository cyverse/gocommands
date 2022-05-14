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
	Use:   "goput",
	Short: "Upload iRODS data-objects or collections",
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
		// download to current dir
		err = putOne(filesystem, args[0], "./")
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 2 {
		irodsPath := args[len(args)-1]
		for _, objPath := range args[:len(args)-1] {
			err = putOne(filesystem, objPath, irodsPath)
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

func putOne(filesystem *irodsclient_fs.FileSystem, objPath string, targetPath string) error {
	objPath = commons.MakeLocalPath(objPath)
	cwd := commons.GetCWD()
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	st, err := os.Stat(objPath)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return putDataObject(filesystem, objPath, targetPath)
	} else {
		// dir
		entries, err := os.ReadDir(objPath)
		if err != nil {
			return err
		}

		// make parent dir
		err = filesystem.MakeDir(filepath.Join(targetPath, filepath.Base(objPath)), true)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {

			return putOne(filesystem, filepath.Join(objPath, entryInDir.Name()), filepath.Join(targetPath, entryInDir.Name()))
		}
	}
	return nil
}

func putDataObject(filesystem *irodsclient_fs.FileSystem, objPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putDataObject",
	})

	logger.Debugf("uploading a file %s to an iRODS collection %s\n", objPath, targetPath)

	st, err := os.Stat(objPath)
	if err != nil {
		return err
	}

	threadNum := calcThreadNum(st.Size())
	err = filesystem.UploadFileParallel(objPath, "", targetPath, threadNum, true)
	if err != nil {
		return err
	}

	return nil
}

func calcThreadNum(size int64) int {
	mb := int64(1024 * 1024)
	if size < 5*mb {
		return 1
	} else if size < 40*mb {
		return 2
	} else if size < 100*mb {
		return 3
	}
	return 4
}

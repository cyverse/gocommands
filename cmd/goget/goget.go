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
	Use:   "goget",
	Short: "Download an iRODS data-object",
	Long:  `This downloads an iRODS data-object on the given path to the local disk.`,
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

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-get")
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) == 1 {
		// download to current dir
		err = getOne(filesystem, args[0], "./")
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 2 {
		localPath := args[len(args)-1]
		for _, objPath := range args[:len(args)-1] {
			err = getOne(filesystem, objPath, localPath)
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

func getOne(filesystem *irodsclient_fs.FileSystem, objPath string, targetPath string) error {
	entry, err := filesystem.StatFile(objPath)
	if err != nil {
		return err
	}

	if entry.Type == irodsclient_fs.FileEntry {
		return getDataObject(filesystem, objPath, targetPath)
	} else {
		// dir
		entries, err := filesystem.List(entry.Path)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			return getOne(filesystem, entryInDir.Path, filepath.Join(targetPath, entry.Name))
		}
	}
	return nil
}

func getDataObject(filesystem *irodsclient_fs.FileSystem, objPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getDataObject",
	})

	cwd := commons.GetCWD()
	objPath = commons.MakeIRODSPath(cwd, objPath)
	targetPath = commons.MakeLocalPath(targetPath)

	logger.Debugf("downloading a data object %s to a local dir %s\n", objPath, targetPath)
	// make parent dir
	err := os.MkdirAll(filepath.Dir(targetPath), 0666)
	if err != nil {
		return err
	}

	entry, err := filesystem.StatFile(objPath)
	if err != nil {
		return err
	}

	threadNum := calcThreadNum(entry.Size)
	err = filesystem.DownloadFileParallel(objPath, "", targetPath, threadNum)
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

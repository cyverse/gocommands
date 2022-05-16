package main

import (
	"fmt"
	"os"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gormdir [collection1] [collection2] ...",
	Short: "Remove iRODS collections",
	Long:  `This removes iRODS collections.`,
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

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-rmdir")
	if err != nil {
		return err
	}

	defer filesystem.Release()

	for _, targetPath := range args {
		err = removeOne(filesystem, targetPath)
		if err != nil {
			logger.Error(err)
			return err
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

func removeOne(filesystem *irodsclient_fs.FileSystem, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeOne",
	})

	cwd := commons.GetCWD()
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	sourceEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		return fmt.Errorf("%s is not a collection", targetPath)
	} else {
		// dir
		logger.Debugf("removing a collection %s\n", targetPath)
		err = filesystem.RemoveDir(targetPath, false, false)
		if err != nil {
			return err
		}
	}
	return nil
}

package subcmd

import (
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:   "mv [data-object1] [data-object2] [collection1] ... [target collection]",
	Short: "Move iRODS data-objects or collections to target collection, or rename data-object or collection",
	Long:  `This moves iRODS data-objects or collections to the given target collection, or rename a single data-object or collection.`,
	RunE:  processMvCommand,
}

func AddMvCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(mvCmd)

	rootCmd.AddCommand(mvCmd)
}

func processMvCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processMvCommand",
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
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) == 2 {
		// rename or move
		err = moveOne(filesystem, args[0], args[1])
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 3 {
		// move
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = moveOne(filesystem, sourcePath, targetPath)
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

func moveOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "moveOne",
	})

	cwd := commons.GetCWD()
	sourcePath = commons.MakeIRODSPath(cwd, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("renaming a data object %s to %s\n", sourcePath, targetPath)
		err = filesystem.RenameFile(sourcePath, targetPath)
		if err != nil {
			return err
		}
	} else {
		// dir
		logger.Debugf("renaming a collection %s to %s\n", sourcePath, targetPath)
		err = filesystem.RenameDir(sourcePath, targetPath)
		if err != nil {
			return err
		}
	}
	return nil
}
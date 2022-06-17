package subcmd

import (
	"fmt"
	"os"
	"path/filepath"

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

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
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
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = getOne(filesystem, sourcePath, targetPath)
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

func getOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
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
		return getDataObject(filesystem, sourcePath, targetPath)
	} else {
		// dir
		entries, err := filesystem.List(entry.Path)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := filepath.Join(targetPath, entry.Name)
		err = os.MkdirAll(targetDir, 0766)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			err = getOne(filesystem, entryInDir.Path, targetDir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getDataObject(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getDataObject",
	})

	logger.Debugf("downloading a data object %s to a local dir %s\n", sourcePath, targetPath)

	err := filesystem.DownloadFileParallel(sourcePath, "", targetPath, 0)
	if err != nil {
		return err
	}
	return nil
}

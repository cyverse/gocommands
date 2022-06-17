package subcmd

import (
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmdirCmd = &cobra.Command{
	Use:   "rmdir [collection1] [collection2] ...",
	Short: "Remove iRODS collections",
	Long:  `This removes iRODS collections.`,
	RunE:  processRmdirCommand,
}

func AddRmdirCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(rmdirCmd)

	rootCmd.AddCommand(rmdirCmd)
}

func processRmdirCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processRmdirCommand",
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

	if len(args) == 0 {
		return fmt.Errorf("arguments given are not sufficent")
	}

	for _, targetPath := range args {
		err = removeDirOne(filesystem, targetPath)
		if err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

func removeDirOne(filesystem *irodsclient_fs.FileSystem, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeDirOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		return fmt.Errorf("%s is not a collection", targetPath)
	} else {
		// dir
		logger.Debugf("removing a collection %s", targetPath)
		err = filesystem.RemoveDir(targetPath, false, false)
		if err != nil {
			return err
		}
	}
	return nil
}

package subcmd

import (
	"fmt"
	"os"

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

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	if len(args) == 0 {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	for _, targetPath := range args {
		err = removeDirOne(filesystem, targetPath)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
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

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return err
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
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

package subcmd

import (
	"fmt"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm [data-object1] [data-object2] [collection1] ...",
	Short: "Remove iRODS data-objects or collections",
	Long:  `This removes iRODS data-objects or collections.`,
	RunE:  processRmCommand,
}

func AddRmCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(rmCmd)
	rmCmd.Flags().BoolP("recurse", "r", false, "Remove non-empty collections")
	rmCmd.Flags().BoolP("force", "f", false, "Remove forcefully")

	rootCmd.AddCommand(rmCmd)
}

func processRmCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processRmCommand",
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

	recurse := false
	recurseFlag := command.Flags().Lookup("recurse")
	if recurseFlag != nil {
		recurse, err = strconv.ParseBool(recurseFlag.Value.String())
		if err != nil {
			recurse = false
		}
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

	if len(args) == 0 {
		return fmt.Errorf("arguments given are not sufficent")
	}

	for _, sourcePath := range args {
		err = removeOne(filesystem, sourcePath, force, recurse)
		if err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

func removeOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, force bool, recurse bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("removing a data object %s", sourcePath)
		err = filesystem.RemoveFile(sourcePath, force)
		if err != nil {
			return err
		}
	} else {
		// dir
		if !recurse {
			return fmt.Errorf("cannot remove a collection, recurse is set")
		}

		logger.Debugf("removing a collection %s", sourcePath)
		err = filesystem.RemoveDir(sourcePath, recurse, force)
		if err != nil {
			return err
		}
	}
	return nil
}

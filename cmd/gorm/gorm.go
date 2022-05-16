package main

import (
	"fmt"
	"os"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gorm [data-object1] [data-object2] [collection1] ...",
	Short: "Remove iRODS data-objects or collections",
	Long:  `This removes iRODS data-objects or collections.`,
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

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-rm")
	if err != nil {
		return err
	}

	defer filesystem.Release()

	for _, sourcePath := range args {
		err = removeOne(filesystem, sourcePath, force, recurse)
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
	rootCmd.Flags().BoolP("recurse", "r", false, "Remove non-empty collections")
	rootCmd.Flags().BoolP("force", "f", false, "Remove forcefully")

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

func removeOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, force bool, recurse bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeOne",
	})

	cwd := commons.GetCWD()
	sourcePath = commons.MakeIRODSPath(cwd, sourcePath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("removing a data object %s\n", sourcePath)
		err = filesystem.RemoveFile(sourcePath, force)
		if err != nil {
			return err
		}
	} else {
		// dir
		if !recurse {
			return fmt.Errorf("cannot remove a collection, recurse is set")
		}

		logger.Debugf("removing a collection %s\n", sourcePath)
		err = filesystem.RemoveDir(sourcePath, recurse, force)
		if err != nil {
			return err
		}
	}
	return nil
}

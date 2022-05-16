package main

import (
	"os"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gomkdir [collection1] [collection2] ...",
	Short: "Make iRODS collections",
	Long:  `This makes iRODS collections.`,
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

	parent := false
	parentFlag := command.Flags().Lookup("parent")
	if parentFlag != nil {
		parent, err = strconv.ParseBool(parentFlag.Value.String())
		if err != nil {
			parent = false
		}
	}

	// Create a file system
	account := commons.GetAccount()

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-mkdir")
	if err != nil {
		return err
	}

	defer filesystem.Release()

	for _, targetPath := range args {
		err = makeOne(filesystem, targetPath, parent)
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
	rootCmd.Flags().BoolP("parents", "p", false, "Make parent collections")

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

func makeOne(filesystem *irodsclient_fs.FileSystem, targetPath string, parent bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "makeOne",
	})

	cwd := commons.GetCWD()
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	logger.Debugf("making a collection %s\n", targetPath)
	err := filesystem.MakeDir(targetPath, parent)
	if err != nil {
		return err
	}
	return nil
}

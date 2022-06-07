package subcmd

import (
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var mkdirCmd = &cobra.Command{
	Use:   "mkdir [collection1] [collection2] ...",
	Short: "Make iRODS collections",
	Long:  `This makes iRODS collections.`,
	RunE:  processMkdirCommand,
}

func AddMkdirCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(mkdirCmd)
	mkdirCmd.Flags().BoolP("parents", "p", false, "Make parent collections")

	rootCmd.AddCommand(mkdirCmd)
}

func processMkdirCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processMkdirCommand",
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
	filesystem, err := commons.GetIRODSFSClient(account)
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

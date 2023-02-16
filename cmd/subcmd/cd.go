package subcmd

import (
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cdCmd = &cobra.Command{
	Use:   "cd [collection1]",
	Short: "Change current working iRODS collection",
	Long:  `This changes current working iRODS collection.`,
	RunE:  processCdCommand,
}

func AddCdCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(cdCmd)

	rootCmd.AddCommand(cdCmd)
}

func processCdCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		return err
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return err
	}

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) > 1 {
		return fmt.Errorf("too many arguments")
	}

	targetPath := ""
	if len(args) == 0 {
		// move to home dir
		targetPath = "~"
	} else if len(args) == 1 {
		targetPath = args[0]
	}

	// cd
	err = changeWorkingDir(filesystem, targetPath)
	if err != nil {
		return err
	}
	return nil
}

func changeWorkingDir(fs *irodsclient_fs.FileSystem, collectionPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "changeWorkingDir",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	collectionPath = commons.MakeIRODSPath(cwd, home, zone, collectionPath)

	connection, err := fs.GetConnection()
	if err != nil {
		return err
	}
	defer fs.ReturnConnection(connection)

	logger.Debugf("changing working dir: %s", collectionPath)

	collection, err := irodsclient_irodsfs.GetCollection(connection, collectionPath)
	if err != nil {
		return err
	}

	if collection.ID <= 0 {
		return fmt.Errorf("collection %s does not exist", collectionPath)
	}

	commons.SetCWD(collectionPath)
	return nil
}

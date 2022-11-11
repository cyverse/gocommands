package subcmd

import (
	"fmt"
	"os"
	"strconv"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_fs "github.com/cyverse/go-irodsclient/irods/fs"
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
	mkdirCmd.Flags().BoolP("parents", "", false, "Make parent collections")

	rootCmd.AddCommand(mkdirCmd)
}

func processMkdirCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processMkdirCommand",
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

	parent := false
	parentFlag := command.Flags().Lookup("parent")
	if parentFlag != nil {
		parent, err = strconv.ParseBool(parentFlag.Value.String())
		if err != nil {
			parent = false
		}
	}

	// Create a connection
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer irodsConn.Disconnect()

	for _, targetPath := range args {
		err = makeOne(irodsConn, targetPath, parent)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}
	return nil
}

func makeOne(connection *irodsclient_conn.IRODSConnection, targetPath string, parent bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "makeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	logger.Debugf("making a collection %s", targetPath)

	err := irodsclient_fs.CreateCollection(connection, targetPath, parent)
	if err != nil {
		return err
	}
	return nil
}

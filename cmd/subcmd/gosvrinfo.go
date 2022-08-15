package subcmd

import (
	"fmt"
	"os"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var svrinfoCmd = &cobra.Command{
	Use:   "svrinfo",
	Short: "Display server information",
	Long:  `This displays server information, such as version.`,
	RunE:  processSvrinfoCommand,
}

func AddSvrinfoCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(svrinfoCmd)

	rootCmd.AddCommand(svrinfoCmd)
}

func processSvrinfoCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSvrinfoCommand",
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

	// Create a connection
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer irodsConn.Disconnect()

	err = displayVersion(irodsConn)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func displayVersion(connection *irodsclient_conn.IRODSConnection) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "displayVersion",
	})

	logger.Debug("displaying version")

	account := connection.GetAccount()
	ver := connection.GetVersion()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendRows([]table.Row{
		{
			"Release Version",
			ver.ReleaseVersion,
		},
		{
			"API Version",
			ver.APIVersion,
		},
		{
			"iRODS Zone",
			account.ClientZone,
		},
	}, table.RowConfig{})
	t.Render()

	return nil
}

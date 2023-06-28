package subcmd

import (
	"os"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var svrinfoCmd = &cobra.Command{
	Use:     "svrinfo",
	Aliases: []string{"server_info"},
	Short:   "Display server information",
	Long:    `This displays server information, such as version.`,
	RunE:    processSvrinfoCommand,
}

func AddSvrinfoCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(svrinfoCmd)

	rootCmd.AddCommand(svrinfoCmd)
}

func processSvrinfoCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	err = displayVersion(account, filesystem)
	if err != nil {
		return xerrors.Errorf("failed to perform svrinfo: %w", err)
	}

	return nil
}

func displayVersion(account *types.IRODSAccount, fs *irodsclient_fs.FileSystem) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "displayVersion",
	})

	logger.Debug("displaying version")

	ver, err := fs.GetServerVersion()
	if err != nil {
		return xerrors.Errorf("failed to get server version: %w", err)
	}

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

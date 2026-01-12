package subcmd

import (
	"os"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var svrinfoCmd = &cobra.Command{
	Use:     "svrinfo",
	Aliases: []string{"server_info"},
	Short:   "Display information about the iRODS server",
	Long:    `This command displays information about the iRODS server, such as its version and configuration.`,
	RunE:    processSvrinfoCommand,
	Args:    cobra.NoArgs,
}

func AddSvrinfoCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(svrinfoCmd, true)

	rootCmd.AddCommand(svrinfoCmd)
}

func processSvrinfoCommand(command *cobra.Command, args []string) error {
	svrInfo, err := NewSvrInfoCommand(command, args)
	if err != nil {
		return err
	}

	return svrInfo.Process()
}

type SvrInfoCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewSvrInfoCommand(command *cobra.Command, args []string) (*SvrInfoCommand, error) {
	svrInfo := &SvrInfoCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	return svrInfo, nil
}

func (svrInfo *SvrInfoCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(svrInfo.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// Create a file system
	svrInfo.account = config.GetSessionConfig().ToIRODSAccount()
	svrInfo.filesystem, err = irods.GetIRODSFSClient(svrInfo.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer svrInfo.filesystem.Release()

	if svrInfo.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(svrInfo.filesystem, svrInfo.commonFlagValues.Timeout)
	}

	// run
	err = svrInfo.displayInfo()
	if err != nil {
		return errors.Wrapf(err, "failed to display server info")
	}

	return nil
}

func (svrInfo *SvrInfoCommand) displayInfo() error {
	ver, err := svrInfo.filesystem.GetServerVersion()
	if err != nil {
		return errors.Wrapf(err, "failed to get server version")
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
			svrInfo.account.ClientZone,
		},
	}, table.RowConfig{})
	t.Render()

	return nil
}

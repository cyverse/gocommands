package subcmd

import (
	"os"

	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"iinit", "configure"},
	Short:   "Initialize gocommands",
	Long:    `This command sets up the iRODS Host and access account for use with other gocommands tools. Once the configuration is set, configuration files are created under the ~/.irods directory. The configuration is fully compatible with that of icommands.`,
	RunE:    processInitCommand,
	Args:    cobra.NoArgs,
}

func AddInitCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(initCmd, false)
	flag.SetInitFlags(initCmd)

	rootCmd.AddCommand(initCmd)
}

func processInitCommand(command *cobra.Command, args []string) error {
	init, err := NewInitCommand(command, args)
	if err != nil {
		return err
	}

	return init.Process()
}

type InitCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues
	initFlagValues   *flag.InitFlagValues

	environmentManager *irodsclient_config.ICommandsEnvironmentManager
	account            *irodsclient_types.IRODSAccount
}

func NewInitCommand(command *cobra.Command, args []string) (*InitCommand, error) {
	init := &InitCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
		initFlagValues:   flag.GetInitFlagValues(),
	}

	return init, nil
}

func (init *InitCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(init.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	init.environmentManager = commons.GetEnvironmentManager()

	// handle local flags
	updated := false
	if init.command.Flags().Changed("config") {
		// set config manually
		_, err = commons.InputMissingFields()
		if err != nil {
			return xerrors.Errorf("failed to input missing fields: %w", err)
		}

		updated = true
	} else {
		updated, err = commons.InputFieldsForInit()
		if err != nil {
			return xerrors.Errorf("failed to input fields: %w", err)
		}
	}

	init.account, err = init.environmentManager.ToIRODSAccount()
	if err != nil {
		return xerrors.Errorf("failed to get iRODS account info from iCommands Environment: %w", err)
	}

	// update PAM TTL
	init.account.PamTTL = init.initFlagValues.PamTTL

	// test connect
	conn, err := commons.GetIRODSConnection(init.account)
	if err != nil {
		return xerrors.Errorf("failed to connect to iRODS server: %w", err)
	}
	defer conn.Disconnect()

	if init.account.AuthenticationScheme.IsPAM() {
		// update pam token
		init.environmentManager.Environment.PAMToken = conn.GetPAMToken()
	}

	if updated {
		// save
		manager := commons.GetEnvironmentManager()
		manager.FixAuthConfiguration()

		err = manager.SetEnvironmentDirPath(irodsclient_config.GetDefaultEnvironmentDirPath())
		if err != nil {
			return xerrors.Errorf("failed to set environment dir path: %w", err)
		}

		err = manager.SaveEnvironment()
		if err != nil {
			return xerrors.Errorf("failed to save iCommands Environment: %w", err)
		}
	} else {
		commons.Println("gocommands is already configured for following account:")
		err = init.PrintAccount()
		if err != nil {
			return xerrors.Errorf("failed to print account info: %w", err)
		}
	}

	return nil
}

func (init *InitCommand) PrintAccount() error {
	envMgr := commons.GetEnvironmentManager()
	if envMgr == nil {
		return xerrors.Errorf("environment is not set")
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendRows([]table.Row{
		{
			"iRODS Host",
			envMgr.Environment.Host,
		},
		{
			"iRODS Port",
			envMgr.Environment.Port,
		},
		{
			"iRODS Zone",
			envMgr.Environment.ZoneName,
		},
		{
			"iRODS Username",
			envMgr.Environment.Username,
		},
		{
			"iRODS Authentication Scheme",
			envMgr.Environment.AuthenticationScheme,
		},
	}, table.RowConfig{})
	t.Render()
	return nil
}

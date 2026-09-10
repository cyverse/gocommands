package subcmd

import (
	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
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
	flag.SetOutputFormatFlags(initCmd, true)

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

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
	initFlagValues         *flag.InitFlagValues

	environmentManager *irodsclient_config.ICommandsEnvironmentManager
	account            *irodsclient_types.IRODSAccount
}

func NewInitCommand(command *cobra.Command, args []string) (*InitCommand, error) {
	init := &InitCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
		initFlagValues:         flag.GetInitFlagValues(),
	}

	return init, nil
}

func (init *InitCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(init.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	init.environmentManager = config.GetEnvironmentManager()
	if err := refreshCredentialsForInit(init.environmentManager, init.command.Flags().Changed("config")); err != nil {
		return err
	}

	// handle local flags
	updated := false
	if init.command.Flags().Changed("config") {
		// set config manually
		_, err = config.InputMissingFields()
		if err != nil {
			return errors.Wrapf(err, "failed to input missing fields")
		}

		updated = true
	} else {
		updated, err = config.InputFieldsForInit()
		if err != nil {
			return errors.Wrapf(err, "failed to input fields")
		}
	}

	init.account, err = init.environmentManager.ToIRODSAccount()
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS account info from iCommands Environment")
	}

	// clear PAM token as it will be overwritten
	init.account.PAMToken = ""

	// update PAM TTL
	init.account.PamTTL = init.initFlagValues.PamTTL

	// test connect
	conn, err := irods.GetIRODSConnection(init.account)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to iRODS server")
	}
	defer conn.Disconnect()

	if init.account.AuthenticationScheme.IsPAM() {
		// update pam token
		init.environmentManager.Environment.PAMToken = conn.GetPAMToken()
	}

	if updated {
		// save
		manager := config.GetEnvironmentManager()
		manager.FixAuthConfiguration()

		err = manager.SetEnvironmentDirPath(irodsclient_config.GetDefaultEnvironmentDirPath())
		if err != nil {
			return errors.Wrapf(err, "failed to set environment directory path")
		}

		err = manager.SaveEnvironment()
		if err != nil {
			return errors.Wrapf(err, "failed to save iCommands Environment")
		}
	} else {
		terminal.Println("gocommands is already configured for following account:")
		err = init.PrintAccount()
		if err != nil {
			return errors.Wrapf(err, "failed to print account info")
		}
	}

	return nil
}

// refreshCredentialsForInit prevents a credential read from .irodsA from
// suppressing the password prompt during initialization. A password explicitly
// supplied in a config file or environment variable remains available for
// non-interactive native authentication. PAM tokens are intentionally cleared:
// init obtains a fresh token after authenticating with the password.
func refreshCredentialsForInit(manager *irodsclient_config.ICommandsEnvironmentManager, configSpecified bool) error {
	// The credentials loaded from the authentication file (~/.irods/.irodsA) are
	// outdated when re-initializing. Clear them, so that the user can re-enter
	// them.
	if !irodsclient_util.ExistLocalFile(manager.PasswordFilePath) {
		return nil
	}

	manager.Environment.Password = ""
	manager.Environment.PAMToken = ""

	// The authentication file overwrites credentials given in the configuration
	// file, so read an explicit native password again.
	if configSpecified {
		fileConfig, err := irodsclient_config.NewConfigFromFile(nil, manager.EnvironmentFilePath)
		if err == nil {
			manager.Environment.Password = fileConfig.Password
		}
	}

	// Allow environment variables to override password values.
	envConfig, err := irodsclient_config.NewConfigFromEnv(manager.Environment)
	if err != nil {
		return errors.Wrapf(err, "failed to load config from environment")
	}

	envConfig.PAMToken = ""
	manager.Environment = envConfig
	return nil
}

func (init *InitCommand) PrintAccount() error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())
	outputFormatterTable := outputFormatter.NewTable("iRODS Account Information")
	outputFormatterTable.SetHeader([]string{"Key", "Value"})

	outputFormatterTable.AppendRows([][]interface{}{
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
	})

	if init.outputFormatFlagValues.Format == format.OutputFormatLegacy {
		init.outputFormatFlagValues.Format = format.OutputFormatTable
	}
	outputFormatter.Render(init.outputFormatFlagValues.Format)

	return nil
}

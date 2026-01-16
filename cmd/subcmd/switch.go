package subcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"
)

var switchEnvCmd = &cobra.Command{
	Use:     "switchenv",
	Aliases: []string{"iswitch", "iswitchenv", "switch"},
	Short:   "Change the current iRODS environment",
	Long:    `This command changes the current iRODS environment.`,
	RunE:    processSwitchEnvCommand,
	Args:    cobra.ExactArgs(1),
}

func AddSwitchEnvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(switchEnvCmd, true)
	flag.SetOutputFormatFlags(switchEnvCmd)

	rootCmd.AddCommand(switchEnvCmd)
}

func processSwitchEnvCommand(command *cobra.Command, args []string) error {
	switchEnv, err := NewSwitchEnvCommand(command, args)
	if err != nil {
		return err
	}

	return switchEnv.Process()
}

type SwitchEnvCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues

	account *irodsclient_types.IRODSAccount

	targetEnv string
}

func NewSwitchEnvCommand(command *cobra.Command, args []string) (*SwitchEnvCommand, error) {
	switchEnv := &SwitchEnvCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
	}

	// target environment
	switchEnv.targetEnv = args[0]

	return switchEnv, nil
}

func (switchEnv *SwitchEnvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(switchEnv.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = switchEnv.switchEnvironment(switchEnv.targetEnv)
	if err != nil {
		return errors.Wrapf(err, "failed to switch environment")
	}

	return nil
}

func (switchEnv *SwitchEnvCommand) switchEnvironment(targetEnv string) error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	dirPath := envMgr.EnvironmentDirPath

	envFiles, err := os.ReadDir(dirPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read environment directory %q", dirPath)
	}

	targetEnvFilePath := ""
	for _, envFile := range envFiles {
		if !envFile.IsDir() && strings.HasSuffix(envFile.Name(), ".json") && envFile.Name() != "irods_environment.json" {
			// environment file
			envFilePath := filepath.Join(dirPath, envFile.Name())

			if targetEnv == envFile.Name() || targetEnv+".json" == envFile.Name() || targetEnv == envFilePath {
				targetEnvFilePath = envFilePath
			}
		}
	}

	if targetEnvFilePath == "" {
		targetEnvFilePath := filepath.Join(dirPath, targetEnv+".json")
		return irodsclient_types.NewFileNotFoundError(targetEnvFilePath)
	}

	// copy to irods_environment.json
	if targetEnvFilePath == irodsclient_config.GetDefaultEnvironmentFilePath() {
		// already set
		return errors.Errorf("environment %q is already the current environment", targetEnv)
	}

	// clear existing files
	os.Remove(irodsclient_config.GetDefaultEnvironmentFilePath())
	os.Remove(irodsclient_config.GetDefaultPasswordFilePath())

	// copy environment file
	envData, err := os.ReadFile(targetEnvFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read environment file %q", targetEnvFilePath)
	}

	err = os.WriteFile(irodsclient_config.GetDefaultEnvironmentFilePath(), envData, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write environment file %q", filepath.Join(dirPath, "irods_environment.json"))
	}

	// load the new environment
	err = envMgr.SetEnvironmentFilePath(irodsclient_config.GetDefaultEnvironmentFilePath())
	if err != nil {
		return errors.Wrapf(err, "failed to set environment file path %q", irodsclient_config.GetDefaultEnvironmentFilePath())
	}

	err = envMgr.Load()
	if err != nil {
		return errors.Wrapf(err, "failed to load environment file %q", irodsclient_config.GetDefaultEnvironmentFilePath())
	}

	// load config from env
	envConfig, err := irodsclient_config.NewConfigFromEnv(envMgr.Environment)
	if err != nil {
		return errors.Wrapf(err, "failed to load config from environment")
	}

	// overwrite
	envMgr.Environment = envConfig

	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	switchEnv.account, err = envMgr.ToIRODSAccount()
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS account info from iCommands Environment")
	}

	// clear PAM token as it will be overwritten
	switchEnv.account.PAMToken = ""

	// test connect
	conn, err := irods.GetIRODSConnection(switchEnv.account)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to iRODS server")
	}
	defer conn.Disconnect()

	if switchEnv.account.AuthenticationScheme.IsPAM() {
		// update pam token
		envMgr.Environment.PAMToken = conn.GetPAMToken()
	}

	// save
	envMgr.FixAuthConfiguration()

	err = envMgr.SaveEnvironment()
	if err != nil {
		return errors.Wrapf(err, "failed to save iCommands Environment")
	}

	err = switchEnv.PrintAccount(envMgr)
	if err != nil {
		return errors.Wrapf(err, "failed to print account info")
	}

	return nil
}

func (switchEnv *SwitchEnvCommand) PrintAccount(envMgr *irodsclient_config.ICommandsEnvironmentManager) error {
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
	outputFormatter.Render(switchEnv.outputFormatFlagValues.Format)

	return nil
}

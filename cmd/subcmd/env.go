package subcmd

import (
	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"ienv"},
	Short:   "Print the current iRODS environment",
	Long:    `This command prints the current iRODS environment settings.`,
	RunE:    processEnvCommand,
	Args:    cobra.NoArgs,
}

func AddEnvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(envCmd, true)
	flag.SetOutputFormatFlags(envCmd)

	rootCmd.AddCommand(envCmd)
}

func processEnvCommand(command *cobra.Command, args []string) error {
	env, err := NewEnvCommand(command, args)
	if err != nil {
		return err
	}

	return env.Process()
}

type EnvCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
}

func NewEnvCommand(command *cobra.Command, args []string) (*EnvCommand, error) {
	env := &EnvCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
	}

	return env, nil
}

func (env *EnvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(env.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = env.printEnvironment()
	if err != nil {
		return errors.Wrapf(err, "failed to print environment")
	}

	return nil
}

func (env *EnvCommand) printEnvironment() error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())
	outputFormatterTable := outputFormatter.NewTable("iRODS Environment")

	outputFormatterTable.SetHeader([]string{"Key", "Value"})

	sessionConfig, err := envMgr.GetSessionConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to get session config")
	}

	outputFormatterTable.AppendRows([][]interface{}{
		{
			"Session Environment File",
			envMgr.SessionFilePath,
		},
		{
			"Environment File",
			envMgr.EnvironmentFilePath,
		},
		{
			"Authentication File",
			envMgr.PasswordFilePath,
		},
		{
			"Host",
			sessionConfig.Host,
		},
		{
			"Port",
			sessionConfig.Port,
		},
		{
			"Zone",
			sessionConfig.ZoneName,
		},
		{
			"Username",
			sessionConfig.Username,
		},
		{
			"Client Zone",
			sessionConfig.ClientZoneName,
		},
		{
			"Client Username",
			sessionConfig.ClientUsername,
		},
		{
			"Default Resource",
			sessionConfig.DefaultResource,
		},
		{
			"Current Working Dir",
			config.GetCWD(),
		},
		{
			"Home",
			config.GetHomeDir(),
		},
		{
			"Default Hash Scheme",
			sessionConfig.DefaultHashScheme,
		},
		{
			"Log Level",
			sessionConfig.LogLevel,
		},
		{
			"Authentication Scheme",
			sessionConfig.AuthenticationScheme,
		},
		{
			"Client Server Negotiation",
			envMgr.Environment.ClientServerNegotiation,
		},
		{
			"Client Server Policy",
			envMgr.Environment.ClientServerPolicy,
		},
		{
			"SSL CA Certification File",
			envMgr.Environment.SSLCACertificateFile,
		},
		{
			"SSL CA Certification Path",
			envMgr.Environment.SSLCACertificatePath,
		},
		{
			"SSL Certificate Chain File",
			envMgr.Environment.SSLCertificateChainFile,
		},
		{
			"SSL Certificate Key File",
			envMgr.Environment.SSLCertificateKeyFile,
		},
		{
			"SSL Verify Server",
			envMgr.Environment.SSLVerifyServer,
		},
		{
			"SSL Encryption Key Size",
			envMgr.Environment.EncryptionKeySize,
		},
		{
			"SSL Encryption Key Algorithm",
			envMgr.Environment.EncryptionAlgorithm,
		},
		{
			"SSL Encryption Salt Size",
			envMgr.Environment.EncryptionSaltSize,
		},
		{
			"SSL Encryption Hash Rounds",
			envMgr.Environment.EncryptionNumHashRounds,
		},
	})
	outputFormatter.Render(env.outputFormatFlagValues.Format)

	return nil
}

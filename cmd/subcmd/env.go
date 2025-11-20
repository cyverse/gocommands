package subcmd

import (
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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

	commonFlagValues *flag.CommonFlagValues
}

func NewEnvCommand(command *cobra.Command, args []string) (*EnvCommand, error) {
	env := &EnvCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	return env, nil
}

func (env *EnvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(env.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	err = env.printEnvironment()
	if err != nil {
		return xerrors.Errorf("failed to print environment: %w", err)
	}

	return nil
}

func (env *EnvCommand) printEnvironment() error {
	envMgr := commons.GetEnvironmentManager()
	if envMgr == nil {
		return xerrors.Errorf("environment is not set")
	}

	t := table.NewWriter()
	t.SetOutputMirror(commons.GetTerminalWriter())

	sessionConfig, err := envMgr.GetSessionConfig()
	if err != nil {
		return err
	}

	t.AppendRows([]table.Row{
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
			commons.GetCWD(),
		},
		{
			"Home",
			commons.GetHomeDir(),
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
	}, table.RowConfig{})
	t.Render()

	return nil
}

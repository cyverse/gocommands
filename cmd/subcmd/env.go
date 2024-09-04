package subcmd

import (
	"os"

	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"ienv"},
	Short:   "Print current irods environment",
	Long:    `This prints out current irods environment.`,
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
}

func NewEnvCommand(command *cobra.Command, args []string) (*EnvCommand, error) {
	env := &EnvCommand{
		command: command,
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
	t.SetOutputMirror(os.Stdout)

	t.AppendRows([]table.Row{
		{
			"iRODS Session Environment File",
			envMgr.GetSessionFilePath(os.Getppid()),
		},
		{
			"iRODS Environment File",
			envMgr.GetEnvironmentFilePath(),
		},
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
			envMgr.Environment.Zone,
		},
		{
			"iRODS Username",
			envMgr.Environment.Username,
		},
		{
			"iRODS Default Resource",
			envMgr.Environment.DefaultResource,
		},
		{
			"iRODS Default Hash Scheme",
			envMgr.Environment.DefaultHashScheme,
		},
		{
			"iRODS Authentication Scheme",
			envMgr.Environment.AuthenticationScheme,
		},
		{
			"iRODS Client Server Negotiation",
			envMgr.Environment.ClientServerNegotiation,
		},
		{
			"iRODS Client Server Policy",
			envMgr.Environment.ClientServerPolicy,
		},
		{
			"iRODS SSL CA Certification File",
			envMgr.Environment.SSLCACertificateFile,
		},
		{
			"iRODS SSL CA Certification Path",
			envMgr.Environment.SSLCACertificatePath,
		},
		{
			"iRODS SSL Verify Server",
			envMgr.Environment.SSLVerifyServer,
		},
		{
			"iRODS SSL Encryption Key Size",
			envMgr.Environment.EncryptionKeySize,
		},
		{
			"iRODS SSL Encryption Key Algorithm",
			envMgr.Environment.EncryptionAlgorithm,
		},
		{
			"iRODS SSL Encryption Salt Size",
			envMgr.Environment.EncryptionSaltSize,
		},
		{
			"iRODS SSL Encryption Hash Rounds",
			envMgr.Environment.EncryptionNumHashRounds,
		},
	}, table.RowConfig{})
	t.Render()

	return nil
}

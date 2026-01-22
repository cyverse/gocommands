package subcmd

import (
	"path/filepath"

	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/spf13/cobra"
)

var saveenvCmd = &cobra.Command{
	Use:     "saveenv",
	Aliases: []string{"isave", "isaveenv", "save"},
	Short:   "Save the current iRODS environment",
	Long:    `This command saves the current iRODS environment.`,
	RunE:    processSaveenvCommand,
	Args:    cobra.ExactArgs(1),
}

func AddSaveenvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(saveenvCmd, true)

	rootCmd.AddCommand(saveenvCmd)
}

func processSaveenvCommand(command *cobra.Command, args []string) error {
	saveEnv, err := NewSaveEnvCommand(command, args)
	if err != nil {
		return err
	}

	return saveEnv.Process()
}

type SaveEnvCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues

	account *irodsclient_types.IRODSAccount

	envName string
}

func NewSaveEnvCommand(command *cobra.Command, args []string) (*SaveEnvCommand, error) {
	saveEnv := &SaveEnvCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	// target environment
	saveEnv.envName = args[0]

	return saveEnv, nil
}

func (saveEnv *SaveEnvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(saveEnv.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = saveEnv.saveEnvironment(saveEnv.envName)
	if err != nil {
		return errors.Wrapf(err, "failed to save environment")
	}

	return nil
}

func (saveEnv *SaveEnvCommand) saveEnvironment(envName string) error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	sessionConfig, err := envMgr.GetSessionConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to get session config")
	}

	targetEnvFilePath := filepath.Join(envMgr.EnvironmentDirPath, config.MakeEnvFileName(envName))

	err = sessionConfig.ToFile(targetEnvFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to save environment file %q", targetEnvFilePath)
	}

	return nil
}

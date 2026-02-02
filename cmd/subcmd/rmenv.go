package subcmd

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/spf13/cobra"
)

var rmenvCmd = &cobra.Command{
	Use:     "rmenv",
	Aliases: []string{"irmenv", "rmenv"},
	Short:   "Remove an iRODS environment",
	Long:    `This command removes an iRODS environment.`,
	RunE:    processRmenvCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmenvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmenvCmd, true)

	rootCmd.AddCommand(rmenvCmd)
}

func processRmenvCommand(command *cobra.Command, args []string) error {
	rmEnv, err := NewRmEnvCommand(command, args)
	if err != nil {
		return err
	}

	return rmEnv.Process()
}

type RmEnvCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues

	account *irodsclient_types.IRODSAccount

	envNames []string
}

func NewRmEnvCommand(command *cobra.Command, args []string) (*RmEnvCommand, error) {
	rmEnv := &RmEnvCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	// target environments
	rmEnv.envNames = args

	return rmEnv, nil
}

func (rmEnv *RmEnvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rmEnv.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	for _, envName := range rmEnv.envNames {
		err = rmEnv.removeEnvironment(envName)
		if err != nil {
			return errors.Wrapf(err, "failed to remove environment %q", envName)
		}
	}

	return nil
}

func (rmEnv *RmEnvCommand) removeEnvironment(envName string) error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	targetEnvFilePath := filepath.Join(envMgr.EnvironmentDirPath, config.MakeEnvFileName(envName))
	_, err := os.Stat(targetEnvFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			newErr := irodsclient_types.NewFileNotFoundError(targetEnvFilePath)
			return errors.Wrapf(newErr, "environment %q does not exist", envName)
		}

		return errors.Wrapf(err, "failed to stat environment file %q", targetEnvFilePath)
	}

	err = os.Remove(targetEnvFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to remove environment file %q", targetEnvFilePath)
	}

	return nil
}

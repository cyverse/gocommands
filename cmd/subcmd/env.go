package subcmd

import (
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"ienv"},
	Short:   "Print current irods environment",
	Long:    `This prints out current irods environment.`,
	RunE:    processEnvCommand,
}

func AddEnvCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(envCmd)

	rootCmd.AddCommand(envCmd)
}

func processEnvCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	err = commons.PrintEnvironment()
	if err != nil {
		return xerrors.Errorf("failed to print environment: %w", err)
	}
	return nil
}

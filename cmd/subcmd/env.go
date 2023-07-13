package subcmd

import (
	"github.com/cyverse/gocommands/cmd/flag"
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
	Args:    cobra.NoArgs,
}

func AddEnvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(envCmd)

	rootCmd.AddCommand(envCmd)
}

func processEnvCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
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

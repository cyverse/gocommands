package subcmd

import (
	"fmt"

	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var pwdCmd = &cobra.Command{
	Use:     "pwd",
	Aliases: []string{"ipwd"},
	Short:   "Print current working iRODS collection",
	Long:    `This prints current working iRODS collection.`,
	RunE:    processPwdCommand,
}

func AddPwdCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(pwdCmd)

	rootCmd.AddCommand(pwdCmd)
}

func processPwdCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	printCurrentWorkingDir()
	return nil
}

func printCurrentWorkingDir() {
	cwd := commons.GetCWD()
	fmt.Printf("%s\n", cwd)
}

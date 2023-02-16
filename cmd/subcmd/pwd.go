package subcmd

import (
	"fmt"

	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

var pwdCmd = &cobra.Command{
	Use:   "pwd",
	Short: "Print current working iRODS collection",
	Long:  `This prints current working iRODS collection.`,
	RunE:  processPwdCommand,
}

func AddPwdCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(pwdCmd)

	rootCmd.AddCommand(pwdCmd)
}

func processPwdCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		return err
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return err
	}

	err = printCurrentWorkingDir()
	if err != nil {
		return err
	}
	return nil
}

func printCurrentWorkingDir() error {
	cwd := commons.GetCWD()
	fmt.Printf("%s\n", cwd)
	return nil
}

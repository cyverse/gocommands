package subcmd

import (
	"github.com/cyverse/gocommands/cmd/flag"
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
	Args:    cobra.NoArgs,
}

func AddPwdCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(pwdCmd, true)

	rootCmd.AddCommand(pwdCmd)
}

func processPwdCommand(command *cobra.Command, args []string) error {
	pwd, err := NewPwdCommand(command, args)
	if err != nil {
		return err
	}

	return pwd.Process()
}

type PwdCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues
}

func NewPwdCommand(command *cobra.Command, args []string) (*PwdCommand, error) {
	pwd := &PwdCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	return pwd, nil
}

func (pwd *PwdCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(pwd.command)
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

	err = pwd.printCurrentWorkingDir()
	if err != nil {
		return xerrors.Errorf("failed to print current working directory: %w", err)
	}
	return nil
}

func (pwd *PwdCommand) printCurrentWorkingDir() error {
	cwd := commons.GetCWD()
	commons.Printf("%s\n", cwd)

	return nil
}

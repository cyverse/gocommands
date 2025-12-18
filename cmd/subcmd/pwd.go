package subcmd

import (
	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"
)

var pwdCmd = &cobra.Command{
	Use:     "pwd",
	Aliases: []string{"ipwd"},
	Short:   "Print the current working iRODS collection",
	Long:    `This command prints the current working iRODS collection.`,
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
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	err = pwd.printCurrentWorkingDir()
	if err != nil {
		return errors.Wrapf(err, "failed to print current working directory")
	}
	return nil
}

func (pwd *PwdCommand) printCurrentWorkingDir() error {
	cwd := config.GetCWD()
	terminal.Printf("%s\n", cwd)

	return nil
}

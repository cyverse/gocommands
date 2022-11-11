package subcmd

import (
	"fmt"
	"os"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
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
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processPwdCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	err = printCurrentWorkingDir()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}
	return nil
}

func printCurrentWorkingDir() error {
	cwd := commons.GetCWD()
	fmt.Printf("%s\n", cwd)
	return nil
}

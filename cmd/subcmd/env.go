package subcmd

import (
	"fmt"
	"os"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Print current irods environment",
	Long:  `This prints out current irods environment.`,
	RunE:  processEnvCommand,
}

func AddEnvCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(envCmd)

	rootCmd.AddCommand(envCmd)
}

func processEnvCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processEnvCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	err = commons.PrintEnvironment()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}
	return nil
}

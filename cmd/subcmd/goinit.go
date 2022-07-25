package subcmd

import (
	"fmt"
	"os"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gocommands",
	Long: `This sets up iRODS Host and access account for other gocommands tools. 
	Once the configuration is set, configuration files are created under ~/.irods directory.
	The configuration is fully-compatible to that of icommands`,
	RunE: processInitCommand,
}

func AddInitCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(initCmd)

	rootCmd.AddCommand(initCmd)
}

func processInitCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processInitCommand",
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
	updated, err := commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	account, err := commons.GetEnvironmentManager().ToIRODSAccount()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	err = commons.TestConnect(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if updated {
		// save
		err := commons.GetEnvironmentManager().Save()
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	} else {
		fmt.Println("gocommands is already configured for following account:")
		err := commons.PrintAccount(command)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}
	return nil
}

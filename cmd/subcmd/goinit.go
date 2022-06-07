package subcmd

import (
	"fmt"

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
		logger.Error(err)
	}

	if !cont {
		return err
	}

	// handle local flags
	updated, err := commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		return err
	}

	account, err := commons.GetEnvironmentManager().ToIRODSAccount()
	if err != nil {
		logger.Error(err)
		return err
	}

	err = commons.TestConnect(account)
	if err != nil {
		logger.Error(err)
		return err
	}

	if updated {
		// save
		err := commons.GetEnvironmentManager().Save()
		if err != nil {
			logger.Error(err)
			return err
		}
	} else {
		fmt.Println("gocommands is already configured for following account:")
		err := commons.PrintAccount(command)
		if err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

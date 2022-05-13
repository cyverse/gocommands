package main

import (
	"fmt"
	"os"
	"time"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "goinit",
	Short: "Initialize gocommands",
	Long: `This sets up iRODS Host and access account for other gocommands tools. 
	Once the configuration is set, configuration files are created under ~/.irods directory.
	The configuration is fully-compatible to that of icommands`,
	RunE: processCommand,
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
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

	err = testConnect(account)
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

func main() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	commons.SetCommonFlags(rootCmd)

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

// testConnect just test connection creation
func testConnect(account *irodsclient_types.IRODSAccount) error {
	oneMin := 1 * time.Minute
	conn := irodsclient_conn.NewIRODSConnection(account, oneMin, "gocommands-init")

	err := conn.Connect()
	if err != nil {
		return err
	}

	defer conn.Disconnect()
	return nil
}

package main

import (
	"os"

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
	Run: processCommand,
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		logger.Error(err)
	}

	if !cont {
		return
	}

	// handle local flags

	account := commons.GetAccount()
	if account == nil {
		// empty
		logger.Debugf("Account is not set yet")

	} else {
		logger.Printf("Connecting to %s:%d\n", account.Host, account.Port)
	}
}

func main() {
	log.SetLevel(log.DebugLevel)

	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	commons.SetCommonFlags(rootCmd)
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

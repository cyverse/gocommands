package main

import (
	"fmt"
	"os"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/cmd/subcmd"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "gocmd [subcommand]",
	Short:         "Gocommands, a command-line iRODS client",
	Long:          `Gocommands, a command-line iRODS client.`,
	RunE:          processCommand,
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd:   true,
		DisableNoDescFlag:   true,
		DisableDescriptions: true,
		HiddenDefaultCmd:    true,
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
	})

	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		logger.Errorf("%+v", err)
	}

	if !cont {
		return err
	}

	// if nothing is given
	command.Usage()

	return nil
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)

	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	flag.SetCommonFlags(rootCmd)

	// add sub commands
	subcmd.AddInitCommand(rootCmd)
	subcmd.AddEnvCommand(rootCmd)
	subcmd.AddPasswdCommand(rootCmd)
	subcmd.AddPwdCommand(rootCmd)
	subcmd.AddCdCommand(rootCmd)
	subcmd.AddLsCommand(rootCmd)
	subcmd.AddCpCommand(rootCmd)
	subcmd.AddMvCommand(rootCmd)
	subcmd.AddCatCommand(rootCmd)
	subcmd.AddGetCommand(rootCmd)
	subcmd.AddPutCommand(rootCmd)
	subcmd.AddSyncCommand(rootCmd)
	subcmd.AddMkdirCommand(rootCmd)
	subcmd.AddRmCommand(rootCmd)
	subcmd.AddRmdirCommand(rootCmd)
	subcmd.AddBunCommand(rootCmd)
	subcmd.AddBputCommand(rootCmd)
	subcmd.AddSvrinfoCommand(rootCmd)
	subcmd.AddPsCommand(rootCmd)
	subcmd.AddCopySftpIdCommand(rootCmd)
	subcmd.AddLsticketCommand(rootCmd)
	subcmd.AddRmticketCommand(rootCmd)
	subcmd.AddMkticketCommand(rootCmd)
	subcmd.AddModticketCommand(rootCmd)
	subcmd.AddBcleanCommand(rootCmd)
	subcmd.AddUpgradeCommand(rootCmd)

	err := Execute()
	if err != nil {
		logger.Errorf("%+v", err)

		if irodsclient_types.IsConnectionConfigError(err) || irodsclient_types.IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to iRODS server!\n")
		} else if irodsclient_types.IsAuthError(err) {
			fmt.Fprintf(os.Stderr, "Authentication failed!\n")
		} else if irodsclient_types.IsFileNotFoundError(err) {
			fmt.Fprintf(os.Stderr, "File or dir not found!\n")
		}

		fmt.Fprintf(os.Stderr, "\nError Trace:\n  - %+v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"os"

	"github.com/cyverse/gocommands/cmd/subcmd"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gocmd [subcommand]",
	Short: "Gocommands, a command-line iRODS client",
	Long:  `Gocommands, a command-line iRODS client.`,
	RunE:  processCommand,
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

	// if nothing is given
	commons.PrintHelp(command)

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
	commons.SetCommonFlags(rootCmd)

	// add sub commands
	subcmd.AddInitCommand(rootCmd)
	subcmd.AddEnvCommand(rootCmd)
	subcmd.AddPasswdCommand(rootCmd)
	subcmd.AddPwdCommand(rootCmd)
	subcmd.AddCdCommand(rootCmd)
	subcmd.AddLsCommand(rootCmd)
	subcmd.AddCpCommand(rootCmd)
	subcmd.AddMvCommand(rootCmd)
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

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

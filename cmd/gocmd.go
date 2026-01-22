package main

import (
	"errors"
	"os"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/cmd/subcmd"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "gocmd <subcommand> [flags]",
	Short:         "GoCommands: A command-line interface for interacting with iRODS",
	Long:          `Gocommands is a powerful command-line tool for interacting with iRODS (Integrated Rule-Oriented Data System). It allows users to manage data objects, collections, and more within iRODS from the terminal.`,
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
		"command": command.Name(),
		"args":    args,
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
	terminal.InitTerminalOutput()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)
	log.SetReportCaller(true)
	log.SetOutput(terminal.GetTerminalWriter())

	logger := log.WithFields(log.Fields{})

	err := config.InitSystemConfig()
	if err != nil {
		logger.Debugf("failed to init system config: %v", err)
	}

	// attach common flags
	flag.SetCommonFlags(rootCmd, true)

	// add sub commands
	subcmd.AddInitCommand(rootCmd)
	subcmd.AddEnvCommand(rootCmd)
	subcmd.AddLsenvCommand(rootCmd)
	subcmd.AddSaveenvCommand(rootCmd)
	subcmd.AddRmenvCommand(rootCmd)
	subcmd.AddSwitchenvCommand(rootCmd)
	subcmd.AddPasswdCommand(rootCmd)
	subcmd.AddPwdCommand(rootCmd)
	subcmd.AddCdCommand(rootCmd)
	subcmd.AddLsCommand(rootCmd)
	subcmd.AddTouchCommand(rootCmd)
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
	subcmd.AddLsmetaCommand(rootCmd)
	subcmd.AddAddmetaCommand(rootCmd)
	subcmd.AddRmmetaCommand(rootCmd)
	subcmd.AddCopySftpIdCommand(rootCmd)
	subcmd.AddLsticketCommand(rootCmd)
	subcmd.AddRmticketCommand(rootCmd)
	subcmd.AddMkticketCommand(rootCmd)
	subcmd.AddModticketCommand(rootCmd)
	subcmd.AddBcleanCommand(rootCmd)
	subcmd.AddChmodCommand(rootCmd)
	subcmd.AddChmodinheritCommand(rootCmd)
	subcmd.AddUpgradeCommand(rootCmd)

	err = Execute()
	if err != nil {
		logger.Errorf("%+v", err)

		if flag.GetCommonFlagValues(rootCmd).DebugMode {
			terminal.PrintErrorf("%+v\n", err)
		}

		if os.IsNotExist(err) {
			terminal.PrintErrorf("File or directory not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				terminal.PrintErrorf("Failed to establish a connection to iRODS server (host: %q, port: %d)!\n", connectionConfigError.Account.Host, connectionConfigError.Account.Port)
			} else {
				terminal.PrintErrorf("Failed to establish a connection to iRODS server!\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			terminal.PrintErrorf("Failed to establish a connection to iRODS server!\n")
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				terminal.PrintErrorf("Failed to establish a new connection to iRODS server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				terminal.PrintErrorf("Failed to establish a new connection to iRODS server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				terminal.PrintErrorf("Authentication failed (auth scheme: %q, username: %q, zone: %q)!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				terminal.PrintErrorf("Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				terminal.PrintErrorf("File or directory %q is not found!\n", fileNotFoundError.Path)
			} else {
				terminal.PrintErrorf("File or directory is not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				terminal.PrintErrorf("Directory %q is not empty!\n", collectionNotEmptyError.Path)
			} else {
				terminal.PrintErrorf("Directory is not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				terminal.PrintErrorf("File or directory %q already exists!\n", fileAlreadyExistError.Path)
			} else {
				terminal.PrintErrorf("File or directory already exists!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				terminal.PrintErrorf("Ticket %q is not found!\n", ticketNotFoundError.Ticket)
			} else {
				terminal.PrintErrorf("Ticket is not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				terminal.PrintErrorf("User %q is not found!\n", userNotFoundError.Name)
			} else {
				terminal.PrintErrorf("User is not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				terminal.PrintErrorf("iRODS Error (code: '%d', message: %q)\n", irodsError.Code, irodsError.Error())
			} else {
				terminal.PrintErrorf("iRODS Error!\n")
			}
		} else if types.IsNotDirError(err) {
			var notDirError *types.NotDirError
			if errors.As(err, &notDirError) {
				terminal.PrintErrorf("Destination %q is not a directory!\n", notDirError.Path)
			} else {
				terminal.PrintErrorf("Destination is not a directory!\n")
			}
		} else if types.IsNotFileError(err) {
			var notFileError *types.NotFileError
			if errors.As(err, &notFileError) {
				terminal.PrintErrorf("Destination %q is not a file!\n", notFileError.Path)
			} else {
				terminal.PrintErrorf("Destination is not a file!\n")
			}
		} else {
			terminal.PrintErrorf("Unexpected error!\nError Trace:\n  - %+v\n", err)
		}

		os.Exit(1)
	}
}

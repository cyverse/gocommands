package main

import (
	"errors"
	"os"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/cmd/subcmd"
	"github.com/cyverse/gocommands/commons"
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
	commons.InitTerminalOutput()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)
	log.SetOutput(commons.GetTerminalWriter())

	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	flag.SetCommonFlags(rootCmd, true)

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
	subcmd.AddLsmetaCommand(rootCmd)
	subcmd.AddAddmetaCommand(rootCmd)
	subcmd.AddRmmetaCommand(rootCmd)
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

		if os.IsNotExist(err) {
			commons.PrintErrorf("File or directory not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				commons.PrintErrorf("Failed to establish a connection to iRODS server (host: %q, port: %d)!\n", connectionConfigError.Config.Host, connectionConfigError.Config.Port)
			} else {
				commons.PrintErrorf("Failed to establish a connection to iRODS server!\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			commons.PrintErrorf("Failed to establish a connection to iRODS server!\n")
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				commons.PrintErrorf("Failed to establish a new connection to iRODS server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				commons.PrintErrorf("Failed to establish a new connection to iRODS server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				commons.PrintErrorf("Authentication failed (auth scheme: %q, username: %q, zone: %q)!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				commons.PrintErrorf("Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				commons.PrintErrorf("File or directory %q is not found!\n", fileNotFoundError.Path)
			} else {
				commons.PrintErrorf("File or directory is not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				commons.PrintErrorf("Directory %q is not empty!\n", collectionNotEmptyError.Path)
			} else {
				commons.PrintErrorf("Directory is not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				commons.PrintErrorf("File or directory %q already exists!\n", fileAlreadyExistError.Path)
			} else {
				commons.PrintErrorf("File or directory already exists!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				commons.PrintErrorf("Ticket %q is not found!\n", ticketNotFoundError.Ticket)
			} else {
				commons.PrintErrorf("Ticket is not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				commons.PrintErrorf("User %q is not found!\n", userNotFoundError.Name)
			} else {
				commons.PrintErrorf("User is not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				commons.PrintErrorf("iRODS Error (code: '%d', message: %q)\n", irodsError.Code, irodsError.Error())
			} else {
				commons.PrintErrorf("iRODS Error!\n")
			}
		} else if commons.IsNotDirError(err) {
			var notDirError *commons.NotDirError
			if errors.As(err, &notDirError) {
				commons.PrintErrorf("Destination %q is not a directory!\n", notDirError.Path)
			} else {
				commons.PrintErrorf("Destination is not a directory!\n")
			}
		} else {
			commons.PrintErrorf("Unexpected error!\nError Trace:\n  - %+v\n", err)
		}

		os.Exit(1)
	}
}

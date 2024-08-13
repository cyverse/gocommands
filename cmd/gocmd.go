package main

import (
	"errors"
	"fmt"
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
			fmt.Fprintf(os.Stderr, "File or directory not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				fmt.Fprintf(os.Stderr, "Failed to establish a connection to iRODS server (host: %q, port: %d)!\n", connectionConfigError.Config.Host, connectionConfigError.Config.Port)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to establish a connection to iRODS server!\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to iRODS server!\n")
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				fmt.Fprintf(os.Stderr, "Failed to establish a new connection to iRODS server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to establish a new connection to iRODS server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				fmt.Fprintf(os.Stderr, "Authentication failed (auth scheme: %q, username: %q, zone: %q)!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				fmt.Fprintf(os.Stderr, "Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				fmt.Fprintf(os.Stderr, "File or directory %q is not found!\n", fileNotFoundError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "File or directory is not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				fmt.Fprintf(os.Stderr, "Directory %q is not empty!\n", collectionNotEmptyError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "Directory is not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				fmt.Fprintf(os.Stderr, "File or directory %q already exists!\n", fileAlreadyExistError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "File or directory already exists!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				fmt.Fprintf(os.Stderr, "Ticket %q is not found!\n", ticketNotFoundError.Ticket)
			} else {
				fmt.Fprintf(os.Stderr, "Ticket is not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				fmt.Fprintf(os.Stderr, "User %q is not found!\n", userNotFoundError.Name)
			} else {
				fmt.Fprintf(os.Stderr, "User is not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				fmt.Fprintf(os.Stderr, "iRODS Error (code: '%d', message: %q)\n", irodsError.Code, irodsError.Error())
			} else {
				fmt.Fprintf(os.Stderr, "iRODS Error!\n")
			}
		} else if commons.IsNotDirError(err) {
			var notDirError *commons.NotDirError
			if errors.As(err, &notDirError) {
				fmt.Fprintf(os.Stderr, "Destination %q is not a director!\n", notDirError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "Destination is not a director!\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected error!\nError Trace:\n  - %+v\n", err)
		}

		//fmt.Fprintf(os.Stderr, "\nError Trace:\n  - %+v\n", err)
		os.Exit(1)
	}
}

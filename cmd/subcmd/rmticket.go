package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rmticketCmd = &cobra.Command{
	Use:     "rmticket [ticket_string1] [ticket_string2] ...",
	Aliases: []string{"rm_ticket", "remove_ticket"},
	Short:   "Remove tickets for the user",
	Long:    `This removes tickets for the user.`,
	RunE:    processRmticketCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmticketCmd)

	rootCmd.AddCommand(rmticketCmd)
}

func processRmticketCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	for _, ticketName := range args {
		err = removeTicket(filesystem, ticketName)
		if err != nil {
			return xerrors.Errorf("failed to perform remove ticket %s: %w", ticketName, err)
		}
	}
	return nil
}

func removeTicket(fs *irodsclient_fs.FileSystem, ticketName string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeTicket",
	})

	logger.Debugf("remove ticket: %s", ticketName)

	err := fs.DeleteTicket(ticketName)
	if err != nil {
		return xerrors.Errorf("failed to delete ticket %s: %w", ticketName, err)
	}

	return nil
}

package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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
	flag.SetCommonFlags(rmticketCmd, true)

	rootCmd.AddCommand(rmticketCmd)
}

func processRmticketCommand(command *cobra.Command, args []string) error {
	rmTicket, err := NewRmTicketCommand(command, args)
	if err != nil {
		return err
	}

	return rmTicket.Process()
}

type RmTicketCommand struct {
	command *cobra.Command

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	tickets []string
}

func NewRmTicketCommand(command *cobra.Command, args []string) (*RmTicketCommand, error) {
	rmTicket := &RmTicketCommand{
		command: command,
	}

	// tickets
	rmTicket.tickets = args

	return rmTicket, nil
}

func (rmTicket *RmTicketCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rmTicket.command)
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
	rmTicket.account = commons.GetAccount()
	rmTicket.filesystem, err = commons.GetIRODSFSClient(rmTicket.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer rmTicket.filesystem.Release()

	for _, ticketName := range rmTicket.tickets {
		err = rmTicket.removeTicket(ticketName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rmTicket *RmTicketCommand) removeTicket(ticketName string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmTicketCommand",
		"function": "removeTicket",
	})

	logger.Debugf("remove ticket %q", ticketName)

	err := rmTicket.filesystem.DeleteTicket(ticketName)
	if err != nil {
		return xerrors.Errorf("failed to delete ticket %q: %w", ticketName, err)
	}

	return nil
}

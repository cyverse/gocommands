package subcmd

import (
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmticketCmd = &cobra.Command{
	Use:     "rmticket <ticket-name-or-id>...",
	Aliases: []string{"rm_ticket", "remove_ticket"},
	Short:   "Remove tickets for a user",
	Long:    `This command removes one or more tickets for the specified user.`,
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

	commonFlagValues *flag.CommonFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	tickets []string
}

func NewRmTicketCommand(command *cobra.Command, args []string) (*RmTicketCommand, error) {
	rmTicket := &RmTicketCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	// tickets
	rmTicket.tickets = args

	return rmTicket, nil
}

func (rmTicket *RmTicketCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rmTicket.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// Create a file system
	rmTicket.account = config.GetSessionConfig().ToIRODSAccount()
	rmTicket.filesystem, err = irods.GetIRODSFSClient(rmTicket.account, true, false)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer rmTicket.filesystem.Release()

	if rmTicket.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(rmTicket.filesystem, rmTicket.commonFlagValues.Timeout)
	}

	for _, ticketName := range rmTicket.tickets {
		err = rmTicket.removeTicket(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to remove ticket %q: %w", ticketName, err)
		}
	}
	return nil
}

func (rmTicket *RmTicketCommand) removeTicket(ticketName string) error {
	logger := log.WithFields(log.Fields{
		"ticket_name": ticketName,
	})

	logger.Debug("remove a ticket")

	err := rmTicket.filesystem.DeleteTicket(ticketName)
	if err != nil {
		return errors.Wrapf(err, "failed to delete ticket %q", ticketName)
	}

	return nil
}

package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var mkticketCmd = &cobra.Command{
	Use:     "mkticket <collection|data-object>",
	Aliases: []string{"mk_ticket", "make_ticket"},
	Short:   "Create a ticket for a collection or data object",
	Long:    `This command creates a ticket for the specified collection or data object in iRODS.`,
	RunE:    processMkticketCommand,
	Args:    cobra.ExactArgs(1),
}

func AddMkticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(mkticketCmd, true)

	flag.SetTicketFlags(mkticketCmd)

	rootCmd.AddCommand(mkticketCmd)
}

func processMkticketCommand(command *cobra.Command, args []string) error {
	mkTicket, err := NewMkTicketCommand(command, args)
	if err != nil {
		return err
	}

	return mkTicket.Process()
}

type MkTicketCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues
	ticketFlagValues *flag.TicketFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePath string
}

func NewMkTicketCommand(command *cobra.Command, args []string) (*MkTicketCommand, error) {
	mkTicket := &MkTicketCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
		ticketFlagValues: flag.GetTicketFlagValues(),
	}

	// path
	mkTicket.sourcePath = args[0]

	return mkTicket, nil
}

func (mkTicket *MkTicketCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(mkTicket.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	mkTicket.account = config.GetSessionConfig().ToIRODSAccount()
	mkTicket.filesystem, err = irods.GetIRODSFSClient(mkTicket.account, true, false)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer mkTicket.filesystem.Release()

	if mkTicket.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(mkTicket.filesystem, mkTicket.commonFlagValues.Timeout)
	}

	// make ticket
	err = mkTicket.makeTicket(mkTicket.ticketFlagValues.Name, mkTicket.ticketFlagValues.Type, mkTicket.sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to make a ticket for %q, %q, %q: %w", mkTicket.ticketFlagValues.Name, mkTicket.ticketFlagValues.Type, mkTicket.sourcePath, err)
	}
	return nil
}

func (mkTicket *MkTicketCommand) makeTicket(ticketName string, ticketType irodsclient_types.TicketType, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"ticket_name": ticketName,
		"ticket_type": ticketType,
		"target_path": targetPath,
	})

	logger.Debug("create a ticket")

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := mkTicket.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	err := mkTicket.filesystem.CreateTicket(ticketName, ticketType, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create ticket %q: %w", ticketName, err)
	}

	return nil
}

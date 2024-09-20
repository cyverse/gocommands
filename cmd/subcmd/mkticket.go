package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var mkticketCmd = &cobra.Command{
	Use:     "mkticket [collection|data object]",
	Aliases: []string{"mk_ticket", "make_ticket"},
	Short:   "Make a ticket",
	Long:    `This makes a ticket for given collection or data object.`,
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

	ticketFlagValues *flag.TicketFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePath string
}

func NewMkTicketCommand(command *cobra.Command, args []string) (*MkTicketCommand, error) {
	mkTicket := &MkTicketCommand{
		command: command,

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
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	mkTicket.account = commons.GetSessionConfig().ToIRODSAccount()
	mkTicket.filesystem, err = commons.GetIRODSFSClient(mkTicket.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer mkTicket.filesystem.Release()

	// make ticket
	err = mkTicket.makeTicket(mkTicket.ticketFlagValues.Name, mkTicket.ticketFlagValues.Type, mkTicket.sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to make a ticket for %q, %q, %q: %w", mkTicket.ticketFlagValues.Name, mkTicket.ticketFlagValues.Type, mkTicket.sourcePath, err)
	}
	return nil
}

func (mkTicket *MkTicketCommand) makeTicket(ticketName string, ticketType types.TicketType, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "MkTicketCommand",
		"function": "makeTicket",
	})

	logger.Debugf("make ticket %q", ticketName)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := mkTicket.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := mkTicket.filesystem.CreateTicket(ticketName, ticketType, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create ticket %q: %w", ticketName, err)
	}

	return nil
}

package subcmd

import (
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	"github.com/rs/xid"
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
}

func AddMkticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(mkticketCmd)

	mkticketCmd.Flags().StringP("name", "n", "", "Specify ticket name")
	mkticketCmd.Flags().StringP("type", "t", "read", "Specify ticket type (read|write)")

	rootCmd.AddCommand(mkticketCmd)
}

func processMkticketCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
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

	ticketName := xid.New().String()
	ticketNameFlag := command.Flags().Lookup("name")
	if ticketNameFlag != nil {
		ticketName = ticketNameFlag.Value.String()
	}

	ticketType := string(types.TicketTypeRead)
	ticketTypeFlag := command.Flags().Lookup("type")
	if ticketTypeFlag != nil {
		ticketType = ticketTypeFlag.Value.String()
	}

	switch strings.ToLower(ticketType) {
	case string(types.TicketTypeRead), "r":
		ticketType = string(types.TicketTypeRead)
	case string(types.TicketTypeWrite), "w", "rw", "readwrite", "read-write":
		ticketType = string(types.TicketTypeWrite)
	default:
		ticketType = string(types.TicketTypeRead)
	}

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(args) > 1 {
		return xerrors.Errorf("too many arguments")
	}

	if len(args) == 0 {
		return xerrors.Errorf("not enough input arguments")
	}

	err = makeTicket(filesystem, ticketName, types.TicketType(ticketType), args[0])
	if err != nil {
		return xerrors.Errorf("failed to perform make ticket for %s, %s, %s: %w", ticketName, ticketType, args[0], err)
	}
	return nil
}

func makeTicket(fs *irodsclient_fs.FileSystem, ticketName string, ticketType types.TicketType, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "makeTicket",
	})

	logger.Debugf("make ticket: %s", ticketName)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := fs.CreateTicket(ticketName, ticketType, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create ticket %s: %w", ticketName, err)
	}

	return nil
}

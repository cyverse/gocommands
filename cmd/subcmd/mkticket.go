package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
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

	ticketFlagValues := flag.GetTicketFlagValues()

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	err = makeTicket(filesystem, ticketFlagValues.Name, ticketFlagValues.Type, args[0])
	if err != nil {
		return xerrors.Errorf("failed to perform make ticket for %s, %s, %s: %w", ticketFlagValues.Name, ticketFlagValues.Type, args[0], err)
	}
	return nil
}

func makeTicket(fs *irodsclient_fs.FileSystem, ticketName string, ticketType types.TicketType, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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

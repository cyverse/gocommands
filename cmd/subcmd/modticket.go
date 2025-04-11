package subcmd

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var modticketCmd = &cobra.Command{
	Use:     "modticket <ticket-name-or-id>",
	Aliases: []string{"mod_ticket", "modify_ticket", "update_ticket"},
	Short:   "Modify an existing ticket",
	Long:    `This command allows modification of an existing ticket.`,
	RunE:    processModticketCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddModticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(modticketCmd, true)

	flag.SetTicketUpdateFlags(modticketCmd)

	rootCmd.AddCommand(modticketCmd)
}

func processModticketCommand(command *cobra.Command, args []string) error {
	modTicket, err := NewModTicketCommand(command, args)
	if err != nil {
		return err
	}

	return modTicket.Process()
}

type ModTicketCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	ticketUpdateFlagValues *flag.TicketUpdateFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	tickets []string
}

func NewModTicketCommand(command *cobra.Command, args []string) (*ModTicketCommand, error) {
	modTicket := &ModTicketCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		ticketUpdateFlagValues: flag.GetTicketUpdateFlagValues(command),
	}

	modTicket.tickets = args

	return modTicket, nil
}

func (modTicket *ModTicketCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(modTicket.command)
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
	modTicket.account = commons.GetSessionConfig().ToIRODSAccount()
	modTicket.filesystem, err = commons.GetIRODSFSClientForSingleOperation(modTicket.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer modTicket.filesystem.Release()

	for _, ticketName := range modTicket.tickets {
		if modTicket.ticketUpdateFlagValues.UseLimitUpdated {
			err = modTicket.modTicketUseLimit(ticketName, modTicket.ticketUpdateFlagValues.UseLimit)
			if err != nil {
				return err
			}
		}

		if modTicket.ticketUpdateFlagValues.WriteFileLimitUpdated {
			err = modTicket.modTicketWriteFileLimit(ticketName, modTicket.ticketUpdateFlagValues.WriteFileLimit)
			if err != nil {
				return err
			}
		}

		if modTicket.ticketUpdateFlagValues.WriteByteLimitUpdated {
			err = modTicket.modTicketWriteByteLimit(ticketName, modTicket.ticketUpdateFlagValues.WriteByteLimit)
			if err != nil {
				return err
			}
		}

		if modTicket.ticketUpdateFlagValues.ExpirationTimeUpdated {
			err = modTicket.modTicketExpirationTime(ticketName, modTicket.ticketUpdateFlagValues.ExpirationTime)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.AddAllowedUsers) > 0 {
			err = modTicket.modTicketAddAllowedUsers(ticketName, modTicket.ticketUpdateFlagValues.AddAllowedUsers)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.RemoveAllwedUsers) > 0 {
			err = modTicket.modTicketRemoveAllowedUsers(ticketName, modTicket.ticketUpdateFlagValues.RemoveAllwedUsers)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.AddAllowedGroups) > 0 {
			err = modTicket.modTicketAddAllowedGroups(ticketName, modTicket.ticketUpdateFlagValues.AddAllowedGroups)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.RemoveAllowedGroups) > 0 {
			err = modTicket.modTicketRemoveAllowedGroups(ticketName, modTicket.ticketUpdateFlagValues.RemoveAllowedGroups)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.AddAllowedHosts) > 0 {
			err = modTicket.modTicketAddAllowedHosts(ticketName, modTicket.ticketUpdateFlagValues.AddAllowedHosts)
			if err != nil {
				return err
			}
		}

		if len(modTicket.ticketUpdateFlagValues.RemoveAllowedHosts) > 0 {
			err = modTicket.modTicketRemoveAllowedHosts(ticketName, modTicket.ticketUpdateFlagValues.RemoveAllowedHosts)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (modTicket *ModTicketCommand) modTicketUseLimit(ticketName string, ulimit int64) error {
	err := modTicket.filesystem.ModifyTicketUseLimit(ticketName, ulimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify uses limit) %q: %w", ticketName, err)
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketWriteFileLimit(ticketName string, wflimit int64) error {
	err := modTicket.filesystem.ModifyTicketWriteFileLimit(ticketName, wflimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify write file limit) %q: %w", ticketName, err)
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketWriteByteLimit(ticketName string, wblimit int64) error {
	err := modTicket.filesystem.ModifyTicketWriteByteLimit(ticketName, wblimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify write byte limit) %q: %w", ticketName, err)
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketExpirationTime(ticketName string, expiry time.Time) error {
	err := modTicket.filesystem.ModifyTicketExpirationTime(ticketName, expiry)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify expiration time) %q: %w", ticketName, err)
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketAddAllowedUsers(ticketName string, addUsers []string) error {
	for _, addUser := range addUsers {
		err := modTicket.filesystem.AddTicketAllowedUser(ticketName, addUser)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed user) %q: %w", ticketName, err)
		}
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketRemoveAllowedUsers(ticketName string, rmUsers []string) error {
	for _, rmUser := range rmUsers {
		err := modTicket.filesystem.RemoveTicketAllowedUser(ticketName, rmUser)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed user) %q: %w", ticketName, err)
		}
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketAddAllowedGroups(ticketName string, addGroups []string) error {
	for _, addGroup := range addGroups {
		err := modTicket.filesystem.AddTicketAllowedUser(ticketName, addGroup)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed group) %q: %w", ticketName, err)
		}
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketRemoveAllowedGroups(ticketName string, rmGroups []string) error {
	for _, rmGroup := range rmGroups {
		err := modTicket.filesystem.RemoveTicketAllowedUser(ticketName, rmGroup)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed group) %q: %w", ticketName, err)
		}
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketAddAllowedHosts(ticketName string, addHosts []string) error {
	for _, addHost := range addHosts {
		err := modTicket.filesystem.AddTicketAllowedHost(ticketName, addHost)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed host) %q: %w", ticketName, err)
		}
	}
	return nil
}

func (modTicket *ModTicketCommand) modTicketRemoveAllowedHosts(ticketName string, rmHosts []string) error {
	for _, rmHost := range rmHosts {
		err := modTicket.filesystem.RemoveTicketAllowedHost(ticketName, rmHost)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed host) %q: %w", ticketName, err)
		}
	}
	return nil
}

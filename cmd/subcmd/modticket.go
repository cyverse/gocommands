package subcmd

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var modticketCmd = &cobra.Command{
	Use:     "modticket [ticket_name]",
	Aliases: []string{"mod_ticket", "modify_ticket", "update_ticket"},
	Short:   "Modify a ticket",
	Long:    `This modifies a ticket.`,
	RunE:    processModticketCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddModticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(modticketCmd)

	flag.SetTicketFlags(modticketCmd)

	rootCmd.AddCommand(modticketCmd)
}

func processModticketCommand(command *cobra.Command, args []string) error {
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

	ticketUpdateFlagValues := flag.GetTicketUpdateFlagValues(command)

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	for _, ticketName := range args {
		if ticketUpdateFlagValues.UseLimitUpdated {
			err = modTicketUseLimit(filesystem, ticketName, ticketUpdateFlagValues.UseLimit)
			if err != nil {
				return err
			}
		}

		if ticketUpdateFlagValues.WriteFileLimitUpdated {
			err = modTicketWriteFileLimit(filesystem, ticketName, ticketUpdateFlagValues.WriteFileLimit)
			if err != nil {
				return err
			}
		}

		if ticketUpdateFlagValues.WriteByteLimitUpdated {
			err = modTicketWriteByteLimit(filesystem, ticketName, ticketUpdateFlagValues.WriteByteLimit)
			if err != nil {
				return err
			}
		}

		if ticketUpdateFlagValues.ExpirationTimeUpdated {
			err = modTicketExpirationTime(filesystem, ticketName, ticketUpdateFlagValues.ExpirationTime)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.AddAllowedUsers) > 0 {
			err = modTicketAddAllowedUsers(filesystem, ticketName, ticketUpdateFlagValues.AddAllowedUsers)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.RemoveAllwedUsers) > 0 {
			err = modTicketRemoveAllowedUsers(filesystem, ticketName, ticketUpdateFlagValues.RemoveAllwedUsers)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.AddAllowedGroups) > 0 {
			err = modTicketAddAllowedGroups(filesystem, ticketName, ticketUpdateFlagValues.AddAllowedGroups)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.RemoveAllowedGroups) > 0 {
			err = modTicketRemoveAllowedGroups(filesystem, ticketName, ticketUpdateFlagValues.RemoveAllowedGroups)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.AddAllowedHosts) > 0 {
			err = modTicketAddAllowedHosts(filesystem, ticketName, ticketUpdateFlagValues.AddAllowedHosts)
			if err != nil {
				return err
			}
		}

		if len(ticketUpdateFlagValues.RemoveAllowedHosts) > 0 {
			err = modTicketRemoveAllowedHosts(filesystem, ticketName, ticketUpdateFlagValues.RemoveAllowedHosts)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func modTicketUseLimit(fs *irodsclient_fs.FileSystem, ticketName string, ulimit int64) error {
	err := fs.ModifyTicketUseLimit(ticketName, ulimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify uses limit) %s: %w", ticketName, err)
	}
	return nil
}

func modTicketWriteFileLimit(fs *irodsclient_fs.FileSystem, ticketName string, wflimit int64) error {
	err := fs.ModifyTicketWriteFileLimit(ticketName, wflimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify write file limit) %s: %w", ticketName, err)
	}
	return nil
}

func modTicketWriteByteLimit(fs *irodsclient_fs.FileSystem, ticketName string, wblimit int64) error {
	err := fs.ModifyTicketWriteByteLimit(ticketName, wblimit)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify write byte limit) %s: %w", ticketName, err)
	}
	return nil
}

func modTicketExpirationTime(fs *irodsclient_fs.FileSystem, ticketName string, expiry time.Time) error {
	err := fs.ModifyTicketExpirationTime(ticketName, expiry)
	if err != nil {
		return xerrors.Errorf("failed to mod ticket (modify expiration time) %s: %w", ticketName, err)
	}
	return nil
}

func modTicketAddAllowedUsers(fs *irodsclient_fs.FileSystem, ticketName string, addUsers []string) error {
	for _, addUser := range addUsers {
		err := fs.AddTicketAllowedUser(ticketName, addUser)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed user) %s: %w", ticketName, err)
		}
	}
	return nil
}

func modTicketRemoveAllowedUsers(fs *irodsclient_fs.FileSystem, ticketName string, rmUsers []string) error {
	for _, rmUser := range rmUsers {
		err := fs.RemoveTicketAllowedUser(ticketName, rmUser)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed user) %s: %w", ticketName, err)
		}
	}
	return nil
}

func modTicketAddAllowedGroups(fs *irodsclient_fs.FileSystem, ticketName string, addGroups []string) error {
	for _, addGroup := range addGroups {
		err := fs.AddTicketAllowedUser(ticketName, addGroup)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed group) %s: %w", ticketName, err)
		}
	}
	return nil
}

func modTicketRemoveAllowedGroups(fs *irodsclient_fs.FileSystem, ticketName string, rmGroups []string) error {
	for _, rmGroup := range rmGroups {
		err := fs.RemoveTicketAllowedUser(ticketName, rmGroup)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed group) %s: %w", ticketName, err)
		}
	}
	return nil
}

func modTicketAddAllowedHosts(fs *irodsclient_fs.FileSystem, ticketName string, addHosts []string) error {
	for _, addHost := range addHosts {
		err := fs.AddTicketAllowedHost(ticketName, addHost)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (add allowed host) %s: %w", ticketName, err)
		}
	}
	return nil
}

func modTicketRemoveAllowedHosts(fs *irodsclient_fs.FileSystem, ticketName string, rmHosts []string) error {
	for _, rmHost := range rmHosts {
		err := fs.RemoveTicketAllowedHost(ticketName, rmHost)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (remove allowed host) %s: %w", ticketName, err)
		}
	}
	return nil
}

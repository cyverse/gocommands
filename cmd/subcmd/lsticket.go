package subcmd

import (
	"fmt"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var lsticketCmd = &cobra.Command{
	Use:     "lsticket [ticket_string1] [ticket_string2] ...",
	Aliases: []string{"ls_ticket", "list_ticket"},
	Short:   "List tickets for the user",
	Long:    `This lists tickets for the user.`,
	RunE:    processLsticketCommand,
}

func AddLsticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(lsticketCmd)

	lsticketCmd.Flags().BoolP("long", "l", false, "List tickets in a long format")

	rootCmd.AddCommand(lsticketCmd)
}

func processLsticketCommand(command *cobra.Command, args []string) error {
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

	longFormat := false
	longFlag := command.Flags().Lookup("long")
	if longFlag != nil {
		longFormat, err = strconv.ParseBool(longFlag.Value.String())
		if err != nil {
			longFormat = false
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(args) == 0 {
		err = listTicket(filesystem, longFormat)
		if err != nil {
			return xerrors.Errorf("failed to perform list ticket: %w", err)
		}
	} else {
		for _, ticketName := range args {
			err = getTicket(filesystem, ticketName, longFormat)
			if err != nil {
				return xerrors.Errorf("failed to perform get ticket %s: %w", ticketName, err)
			}
		}
	}

	return nil
}

func listTicket(fs *irodsclient_fs.FileSystem, longFormat bool) error {
	tickets, err := fs.ListTickets()
	if err != nil {
		return xerrors.Errorf("failed to list tickets: %w", err)
	}

	if len(tickets) == 0 {
		fmt.Printf("Found no tickets\n")
	} else {
		for _, ticket := range tickets {
			if longFormat {
				restrictions, err := fs.GetTicketRestrictions(ticket.ID)
				if err != nil {
					return xerrors.Errorf("failed to get ticket restrictions %s: %w", ticket.Name, err)
				}

				printTicket(ticket, restrictions)
			} else {
				printTicket(ticket, nil)
			}
		}
	}

	return nil
}

func getTicket(fs *irodsclient_fs.FileSystem, ticketName string, longFormat bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getTicket",
	})

	logger.Debugf("get ticket: %s", ticketName)

	ticket, err := fs.GetTicket(ticketName)
	if err != nil {
		return xerrors.Errorf("failed to get ticket %s: %w", ticketName, err)
	}

	if longFormat {
		restrictions, err := fs.GetTicketRestrictions(ticket.ID)
		if err != nil {
			return xerrors.Errorf("failed to get ticket restrictions %s: %w", ticket.Name, err)
		}

		printTicket(ticket, restrictions)
	} else {
		printTicket(ticket, nil)
	}

	return nil
}

func printTicket(ticket *types.IRODSTicket, restrictions *irodsclient_fs.IRODSTicketRestrictions) {
	fmt.Printf("[%s]\n", ticket.Name)
	fmt.Printf("  id: %d\n", ticket.ID)
	fmt.Printf("  name: %s\n", ticket.Name)
	fmt.Printf("  type: %s\n", ticket.Type)
	fmt.Printf("  owner: %s\n", ticket.Owner)
	fmt.Printf("  owner zone: %s\n", ticket.OwnerZone)
	fmt.Printf("  object type: %s\n", ticket.ObjectType)
	fmt.Printf("  path: %s\n", ticket.Path)
	fmt.Printf("  uses limit: %d\n", ticket.UsesLimit)
	fmt.Printf("  uses count: %d\n", ticket.UsesCount)
	fmt.Printf("  write file limit: %d\n", ticket.WriteFileLimit)
	fmt.Printf("  write file count: %d\n", ticket.WriteFileCount)
	fmt.Printf("  write byte limit: %d\n", ticket.WriteByteLimit)
	fmt.Printf("  write byte count: %d\n", ticket.WriteByteCount)

	if ticket.ExpirationTime.IsZero() {
		fmt.Print("  expiry time: none\n")
	} else {
		fmt.Printf("  expiry time: %s\n", commons.MakeDateTimeString(ticket.ExpirationTime))
	}

	if restrictions != nil {
		if len(restrictions.AllowedHosts) == 0 {
			fmt.Printf("  No host restrictions\n")
		} else {
			for _, host := range restrictions.AllowedHosts {
				fmt.Printf("  Allowed Hosts:\n")
				fmt.Printf("    - %s\n", host)
			}
		}

		if len(restrictions.AllowedUserNames) == 0 {
			fmt.Printf("  No user restrictions\n")
		} else {
			for _, user := range restrictions.AllowedUserNames {
				fmt.Printf("  Allowed Users:\n")
				fmt.Printf("    - %s\n", user)
			}
		}

		if len(restrictions.AllowedGroupNames) == 0 {
			fmt.Printf("  No group restrictions\n")
		} else {
			for _, group := range restrictions.AllowedGroupNames {
				fmt.Printf("  Allowed Groups:\n")
				fmt.Printf("    - %s\n", group)
			}
		}
	}
}

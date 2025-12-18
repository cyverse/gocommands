package subcmd

import (
	"sort"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var lsticketCmd = &cobra.Command{
	Use:     "lsticket <ticket-name-or-id>...",
	Aliases: []string{"ls_ticket", "list_ticket"},
	Short:   "List tickets for the user",
	Long:    `This command lists the tickets associated with the user in iRODS.`,
	RunE:    processLsticketCommand,
	Args:    cobra.ArbitraryArgs,
}

func AddLsticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(lsticketCmd, true)

	flag.SetListFlags(lsticketCmd, true, true)

	rootCmd.AddCommand(lsticketCmd)
}

func processLsticketCommand(command *cobra.Command, args []string) error {
	lsTicket, err := NewLsTicketCommand(command, args)
	if err != nil {
		return err
	}

	return lsTicket.Process()
}

type LsTicketCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues
	listFlagValues   *flag.ListFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	tickets []string
}

func NewLsTicketCommand(command *cobra.Command, args []string) (*LsTicketCommand, error) {
	lsTicket := &LsTicketCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
		listFlagValues:   flag.GetListFlagValues(),
	}

	// tickets
	lsTicket.tickets = args

	return lsTicket, nil
}

func (lsTicket *LsTicketCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(lsTicket.command)
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
	lsTicket.account = config.GetSessionConfig().ToIRODSAccount()
	lsTicket.filesystem, err = irods.GetIRODSFSClient(lsTicket.account, true, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer lsTicket.filesystem.Release()

	if lsTicket.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(lsTicket.filesystem, lsTicket.commonFlagValues.Timeout)
	}

	if len(lsTicket.tickets) == 0 {
		return lsTicket.listTickets()
	}

	for _, ticketName := range lsTicket.tickets {
		err = lsTicket.printTicket(ticketName)
		if err != nil {
			return errors.Wrapf(err, "failed to print ticket %q", ticketName)
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) listTickets() error {
	tickets, err := lsTicket.filesystem.ListTickets()
	if err != nil {
		return errors.Wrapf(err, "failed to list tickets")
	}

	if len(tickets) == 0 {
		terminal.Printf("Found no tickets\n")
	}

	return lsTicket.printTickets(tickets)
}

func (lsTicket *LsTicketCommand) printTicket(ticketName string) error {
	logger := log.WithFields(log.Fields{
		"ticket_name": ticketName,
	})

	logger.Debug("print ticket")

	ticket, err := lsTicket.filesystem.GetTicket(ticketName)
	if err != nil {
		return errors.Wrapf(err, "failed to get ticket %q", ticketName)
	}

	tickets := []*irodsclient_types.IRODSTicket{ticket}
	return lsTicket.printTickets(tickets)
}

func (lsTicket *LsTicketCommand) printTickets(tickets []*irodsclient_types.IRODSTicket) error {
	sort.SliceStable(tickets, lsTicket.getTicketSortFunction(tickets, lsTicket.listFlagValues.SortOrder, lsTicket.listFlagValues.SortReverse))

	for _, ticket := range tickets {
		err := lsTicket.printTicketInternal(ticket)
		if err != nil {
			return errors.Wrapf(err, "failed to print ticket %q", ticket.Name)
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) printTicketInternal(ticket *irodsclient_types.IRODSTicket) error {
	terminal.Printf("[%s]\n", ticket.Name)
	terminal.Printf("  id: %d\n", ticket.ID)
	terminal.Printf("  name: %s\n", ticket.Name)
	terminal.Printf("  type: %s\n", ticket.Type)
	terminal.Printf("  owner: %s\n", ticket.Owner)
	terminal.Printf("  owner zone: %s\n", ticket.OwnerZone)
	terminal.Printf("  object type: %s\n", ticket.ObjectType)
	terminal.Printf("  path: %s\n", ticket.Path)
	terminal.Printf("  uses limit: %d\n", ticket.UsesLimit)
	terminal.Printf("  uses count: %d\n", ticket.UsesCount)
	terminal.Printf("  write file limit: %d\n", ticket.WriteFileLimit)
	terminal.Printf("  write file count: %d\n", ticket.WriteFileCount)
	terminal.Printf("  write byte limit: %d\n", ticket.WriteByteLimit)
	terminal.Printf("  write byte count: %d\n", ticket.WriteByteCount)

	if ticket.ExpirationTime.IsZero() {
		terminal.Print("  expiry time: none\n")
	} else {
		terminal.Printf("  expiry time: %s\n", types.MakeDateTimeString(ticket.ExpirationTime))
	}

	if lsTicket.listFlagValues.Format == format.ListFormatLong || lsTicket.listFlagValues.Format == format.ListFormatVeryLong {
		restrictions, err := lsTicket.filesystem.GetTicketRestrictions(ticket.ID)
		if err != nil {
			return errors.Wrapf(err, "failed to get ticket restrictions %q", ticket.Name)
		}

		if restrictions != nil {
			if len(restrictions.AllowedHosts) == 0 {
				terminal.Printf("  No host restrictions\n")
			} else {
				for _, host := range restrictions.AllowedHosts {
					terminal.Printf("  Allowed Hosts:\n")
					terminal.Printf("    - %s\n", host)
				}
			}

			if len(restrictions.AllowedUserNames) == 0 {
				terminal.Printf("  No user restrictions\n")
			} else {
				for _, user := range restrictions.AllowedUserNames {
					terminal.Printf("  Allowed Users:\n")
					terminal.Printf("    - %s\n", user)
				}
			}

			if len(restrictions.AllowedGroupNames) == 0 {
				terminal.Printf("  No group restrictions\n")
			} else {
				for _, group := range restrictions.AllowedGroupNames {
					terminal.Printf("  Allowed Groups:\n")
					terminal.Printf("    - %s\n", group)
				}
			}
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) getTicketSortFunction(tickets []*irodsclient_types.IRODSTicket, sortOrder format.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case format.ListSortOrderName:
			return func(i int, j int) bool {
				return tickets[i].Name > tickets[j].Name
			}
		case format.ListSortOrderTime:
			return func(i int, j int) bool {
				return (tickets[i].ExpirationTime.After(tickets[j].ExpirationTime)) ||
					(tickets[i].ExpirationTime.Equal(tickets[j].ExpirationTime) &&
						tickets[i].Name < tickets[j].Name)
			}
		// Cannot sort tickets by size or extension, so use default sort by name
		default:
			return func(i int, j int) bool {
				return tickets[i].Name < tickets[j].Name
			}
		}
	}

	switch sortOrder {
	case format.ListSortOrderName:
		return func(i int, j int) bool {
			return tickets[i].Name < tickets[j].Name
		}
	case format.ListSortOrderTime:
		return func(i int, j int) bool {
			return (tickets[i].ExpirationTime.Before(tickets[j].ExpirationTime)) ||
				(tickets[i].ExpirationTime.Equal(tickets[j].ExpirationTime) &&
					tickets[i].Name < tickets[j].Name)

		}
		// Cannot sort tickets by size or extension, so use default sort by name
	default:
		return func(i int, j int) bool {
			return tickets[i].Name < tickets[j].Name
		}
	}
}

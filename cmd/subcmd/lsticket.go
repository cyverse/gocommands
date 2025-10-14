package subcmd

import (
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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
	lsTicket.account = commons.GetSessionConfig().ToIRODSAccount()
	lsTicket.filesystem, err = commons.GetIRODSFSClient(lsTicket.account, true, true)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer lsTicket.filesystem.Release()

	if lsTicket.commonFlagValues.TimeoutUpdated {
		commons.UpdateIRODSFSClientTimeout(lsTicket.filesystem, lsTicket.commonFlagValues.Timeout)
	}

	if len(lsTicket.tickets) == 0 {
		return lsTicket.listTickets()
	}

	for _, ticketName := range lsTicket.tickets {
		err = lsTicket.printTicket(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to print ticket %q: %w", ticketName, err)
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) listTickets() error {
	tickets, err := lsTicket.filesystem.ListTickets()
	if err != nil {
		return xerrors.Errorf("failed to list tickets: %w", err)
	}

	if len(tickets) == 0 {
		commons.Printf("Found no tickets\n")
	}

	return lsTicket.printTickets(tickets)
}

func (lsTicket *LsTicketCommand) printTicket(ticketName string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "LsTicketCommand",
		"function": "printTicket",
	})

	logger.Debugf("print ticket %q", ticketName)

	ticket, err := lsTicket.filesystem.GetTicket(ticketName)
	if err != nil {
		return xerrors.Errorf("failed to get ticket %q: %w", ticketName, err)
	}

	tickets := []*irodsclient_types.IRODSTicket{ticket}
	return lsTicket.printTickets(tickets)
}

func (lsTicket *LsTicketCommand) printTickets(tickets []*irodsclient_types.IRODSTicket) error {
	sort.SliceStable(tickets, lsTicket.getTicketSortFunction(tickets, lsTicket.listFlagValues.SortOrder, lsTicket.listFlagValues.SortReverse))

	for _, ticket := range tickets {
		err := lsTicket.printTicketInternal(ticket)
		if err != nil {
			return xerrors.Errorf("failed to print ticket %q: %w", ticket, err)
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) printTicketInternal(ticket *irodsclient_types.IRODSTicket) error {
	commons.Printf("[%s]\n", ticket.Name)
	commons.Printf("  id: %d\n", ticket.ID)
	commons.Printf("  name: %s\n", ticket.Name)
	commons.Printf("  type: %s\n", ticket.Type)
	commons.Printf("  owner: %s\n", ticket.Owner)
	commons.Printf("  owner zone: %s\n", ticket.OwnerZone)
	commons.Printf("  object type: %s\n", ticket.ObjectType)
	commons.Printf("  path: %s\n", ticket.Path)
	commons.Printf("  uses limit: %d\n", ticket.UsesLimit)
	commons.Printf("  uses count: %d\n", ticket.UsesCount)
	commons.Printf("  write file limit: %d\n", ticket.WriteFileLimit)
	commons.Printf("  write file count: %d\n", ticket.WriteFileCount)
	commons.Printf("  write byte limit: %d\n", ticket.WriteByteLimit)
	commons.Printf("  write byte count: %d\n", ticket.WriteByteCount)

	if ticket.ExpirationTime.IsZero() {
		commons.Print("  expiry time: none\n")
	} else {
		commons.Printf("  expiry time: %s\n", commons.MakeDateTimeString(ticket.ExpirationTime))
	}

	if lsTicket.listFlagValues.Format == commons.ListFormatLong || lsTicket.listFlagValues.Format == commons.ListFormatVeryLong {
		restrictions, err := lsTicket.filesystem.GetTicketRestrictions(ticket.ID)
		if err != nil {
			return xerrors.Errorf("failed to get ticket restrictions %q: %w", ticket.Name, err)
		}

		if restrictions != nil {
			if len(restrictions.AllowedHosts) == 0 {
				commons.Printf("  No host restrictions\n")
			} else {
				for _, host := range restrictions.AllowedHosts {
					commons.Printf("  Allowed Hosts:\n")
					commons.Printf("    - %s\n", host)
				}
			}

			if len(restrictions.AllowedUserNames) == 0 {
				commons.Printf("  No user restrictions\n")
			} else {
				for _, user := range restrictions.AllowedUserNames {
					commons.Printf("  Allowed Users:\n")
					commons.Printf("    - %s\n", user)
				}
			}

			if len(restrictions.AllowedGroupNames) == 0 {
				commons.Printf("  No group restrictions\n")
			} else {
				for _, group := range restrictions.AllowedGroupNames {
					commons.Printf("  Allowed Groups:\n")
					commons.Printf("    - %s\n", group)
				}
			}
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) getTicketSortFunction(tickets []*irodsclient_types.IRODSTicket, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case commons.ListSortOrderName:
			return func(i int, j int) bool {
				return tickets[i].Name > tickets[j].Name
			}
		case commons.ListSortOrderTime:
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
	case commons.ListSortOrderName:
		return func(i int, j int) bool {
			return tickets[i].Name < tickets[j].Name
		}
	case commons.ListSortOrderTime:
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

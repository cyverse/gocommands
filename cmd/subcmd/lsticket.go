package subcmd

import (
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
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
	flag.SetOutputFormatFlags(lsticketCmd)
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

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
	listFlagValues         *flag.ListFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	tickets []string
}

func NewLsTicketCommand(command *cobra.Command, args []string) (*LsTicketCommand, error) {
	lsTicket := &LsTicketCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
		listFlagValues:         flag.GetListFlagValues(),
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
	lsTicket.filesystem, err = irods.GetIRODSFSClient(lsTicket.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer lsTicket.filesystem.Release()

	if lsTicket.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(lsTicket.filesystem, lsTicket.commonFlagValues.Timeout)
	}

	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())
	outputFormatterTable := outputFormatter.NewTable("iRODS Tickets")

	// run
	columns := []string{
		"ID",
		"Name",
		"Type",
		"Owner",
		"Owner Zone",
		"Object Type",
		"Path",
		"Uses Limit",
		"Uses Count",
		"Write File Limit",
		"Write File Count",
		"Write Byte Limit",
		"Write Byte Count",
		"Expiry Time",
	}

	if lsTicket.listFlagValues.Format == format.ListFormatLong || lsTicket.listFlagValues.Format == format.ListFormatVeryLong {
		columns = append(columns,
			"Allowed Hosts",
			"Allowed Users",
			"Allowed Groups",
		)
	}

	outputFormatterTable.SetHeader(columns)

	// run
	tickets := []*irodsclient_types.IRODSTicket{}
	if len(lsTicket.tickets) == 0 {
		tickets, err = lsTicket.filesystem.ListTickets()
		if err != nil {
			return errors.Wrapf(err, "failed to list tickets")
		}
	} else {
		for _, ticketName := range lsTicket.tickets {
			ticket, err := lsTicket.filesystem.GetTicket(ticketName)
			if err != nil {
				return errors.Wrapf(err, "failed to get ticket %q", ticketName)
			}

			tickets = append(tickets, ticket)
		}
	}

	if len(tickets) > 0 {
		err = lsTicket.printTickets(outputFormatterTable, tickets)
		if err != nil {
			return errors.Wrapf(err, "failed to print tickets")
		}
	}

	outputFormatter.Render(lsTicket.outputFormatFlagValues.Format)

	return nil
}

func (lsTicket *LsTicketCommand) printTickets(outputFormatterTable *format.OutputFormatterTable, tickets []*irodsclient_types.IRODSTicket) error {
	sort.SliceStable(tickets, lsTicket.getTicketSortFunction(tickets, lsTicket.listFlagValues.SortOrder, lsTicket.listFlagValues.SortReverse))

	for _, ticket := range tickets {
		err := lsTicket.printTicketInternal(outputFormatterTable, ticket)
		if err != nil {
			return errors.Wrapf(err, "failed to print ticket %q", ticket.Name)
		}
	}

	return nil
}

func (lsTicket *LsTicketCommand) printTicketInternal(outputFormatterTable *format.OutputFormatterTable, ticket *irodsclient_types.IRODSTicket) error {
	expiryTime := "none"
	if !ticket.ExpirationTime.IsZero() {
		expiryTime = types.MakeDateTimeString(ticket.ExpirationTime)
	}

	columnValues := []interface{}{
		ticket.ID,
		ticket.Name,
		ticket.Type,
		ticket.Owner,
		ticket.OwnerZone,
		ticket.ObjectType,
		ticket.Path,
		ticket.UsesLimit,
		ticket.UsesCount,
		ticket.WriteFileLimit,
		ticket.WriteFileCount,
		ticket.WriteByteLimit,
		ticket.WriteByteCount,
		expiryTime,
	}

	if lsTicket.listFlagValues.Format == format.ListFormatLong || lsTicket.listFlagValues.Format == format.ListFormatVeryLong {
		restrictions, err := lsTicket.filesystem.GetTicketRestrictions(ticket.ID)
		if err != nil {
			return errors.Wrapf(err, "failed to get ticket restrictions %q", ticket.Name)
		}

		if restrictions != nil {
			hosts := "any"
			if len(restrictions.AllowedHosts) > 0 {
				hosts = strings.Join(restrictions.AllowedHosts, ", ")
			}

			users := "any"
			if len(restrictions.AllowedUserNames) > 0 {
				users = strings.Join(restrictions.AllowedUserNames, ", ")
			}

			groups := "any"
			if len(restrictions.AllowedGroupNames) > 0 {
				groups = strings.Join(restrictions.AllowedGroupNames, ", ")
			}

			columnValues = append(columnValues,
				hosts,
				users,
				groups,
			)
		}
	}

	outputFormatterTable.AppendRow(columnValues)

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

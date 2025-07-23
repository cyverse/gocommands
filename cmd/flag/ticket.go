package flag

import (
	"strings"
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/rs/xid"
	"github.com/spf13/cobra"
)

type TicketAccessFlagValues struct {
	Name string
}

type TicketFlagValues struct {
	Name      string
	typeInput string
	Type      irodsclient_types.TicketType
}

type TicketUpdateFlagValues struct {
	UseLimit                 int64
	clearUseLimitInput       bool
	UseLimitUpdated          bool
	WriteFileLimit           int64
	clearWriteFileLimitInput bool
	WriteFileLimitUpdated    bool
	WriteByteLimit           int64
	clearWriteByteLimitInput bool
	WriteByteLimitUpdated    bool
	expirationTimeInput      string
	ExpirationTime           time.Time
	ExpirationTimeUpdated    bool
	clearExpirationTimeInput bool
	AddAllowedUsers          []string
	RemoveAllwedUsers        []string
	AddAllowedGroups         []string
	RemoveAllowedGroups      []string
	AddAllowedHosts          []string
	RemoveAllowedHosts       []string
}

var (
	ticketAccessFlagValues TicketAccessFlagValues
	ticketFlagValues       TicketFlagValues
	ticketUpdateFlagValues TicketUpdateFlagValues
)

func SetTicketAccessFlags(command *cobra.Command) {
	command.Flags().StringVarP(&ticketAccessFlagValues.Name, "ticket", "T", "", "Specify the name of the ticket")
}

func GetTicketAccessFlagValues() *TicketAccessFlagValues {
	return &ticketAccessFlagValues
}

func SetTicketFlags(command *cobra.Command) {
	command.Flags().StringVarP(&ticketFlagValues.Name, "name", "n", "", "Specify the name of the ticket")
	command.Flags().StringVarP(&ticketFlagValues.typeInput, "type", "t", "read", "Specify the ticket type (read or write)")
}

func GetTicketFlagValues() *TicketFlagValues {
	if len(ticketFlagValues.Name) == 0 {
		ticketFlagValues.Name = xid.New().String()
	}

	switch strings.ToLower(ticketFlagValues.typeInput) {
	case "read", "r":
		ticketFlagValues.Type = irodsclient_types.TicketTypeRead
	case "write", "w", "rw", "readwrite", "read-write":
		ticketFlagValues.Type = irodsclient_types.TicketTypeWrite
	default:
		ticketFlagValues.Type = irodsclient_types.TicketTypeRead
	}

	return &ticketFlagValues
}

func SetTicketUpdateFlags(command *cobra.Command) {
	command.Flags().Int64Var(&ticketUpdateFlagValues.UseLimit, "ulimit", 0, "Set the usage limit")
	command.Flags().BoolVar(&ticketUpdateFlagValues.clearUseLimitInput, "clear_ulimit", false, "Clear the usage limit")
	command.Flags().Int64Var(&ticketUpdateFlagValues.WriteFileLimit, "wflimit", 0, "Set the write file limit")
	command.Flags().BoolVar(&ticketUpdateFlagValues.clearWriteFileLimitInput, "clear_wflimit", false, "Clear the write file limit")
	command.Flags().Int64Var(&ticketUpdateFlagValues.WriteByteLimit, "wblimit", 0, "Set the write byte limit")
	command.Flags().BoolVar(&ticketUpdateFlagValues.clearWriteByteLimitInput, "clear_wblimit", false, "Clear the write byte limit")
	command.Flags().StringVar(&ticketUpdateFlagValues.expirationTimeInput, "expiry", "0", "Set the expiration time [YYYY-MM-DD HH:mm:SS]")
	command.Flags().BoolVar(&ticketUpdateFlagValues.clearExpirationTimeInput, "clear_expiry", false, "Clear the expiration time")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.AddAllowedUsers, "add_user", []string{}, "Add users to the allowed list")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.AddAllowedGroups, "add_group", []string{}, "Add groups to the allowed list")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.AddAllowedHosts, "add_host", []string{}, "Add hosts to the allowed list")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.RemoveAllwedUsers, "rm_user", []string{}, "Remove users from the allowed list")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.RemoveAllowedGroups, "rm_group", []string{}, "Remove groups from the allowed list")
	command.Flags().StringSliceVar(&ticketUpdateFlagValues.RemoveAllowedHosts, "rm_host", []string{}, "Remove hosts from the allowed list")

	command.MarkFlagsMutuallyExclusive("ulimit", "clear_ulimit")
	command.MarkFlagsMutuallyExclusive("wflimit", "clear_wflimit")
	command.MarkFlagsMutuallyExclusive("wblimit", "clear_wblimit")
	command.MarkFlagsMutuallyExclusive("expiry", "clear_expiry")
}

func GetTicketUpdateFlagValues(command *cobra.Command) *TicketUpdateFlagValues {
	if ticketUpdateFlagValues.clearUseLimitInput {
		ticketUpdateFlagValues.UseLimit = 0
	}

	if command.Flags().Changed("ulimit") || command.Flags().Changed("clear_ulimit") {
		ticketUpdateFlagValues.UseLimitUpdated = true
	}

	if ticketUpdateFlagValues.clearWriteFileLimitInput {
		ticketUpdateFlagValues.WriteFileLimit = 0
	}

	if command.Flags().Changed("wflimit") || command.Flags().Changed("clear_wflimit") {
		ticketUpdateFlagValues.WriteFileLimitUpdated = true
	}

	if ticketUpdateFlagValues.clearWriteByteLimitInput {
		ticketUpdateFlagValues.WriteByteLimit = 0
	}

	if command.Flags().Changed("wblimit") || command.Flags().Changed("clear_wblimit") {
		ticketUpdateFlagValues.WriteByteLimitUpdated = true
	}

	if ticketUpdateFlagValues.clearExpirationTimeInput {
		ticketUpdateFlagValues.ExpirationTime = time.Time{}
	} else {
		exp, err := types.MakeDateTimeFromString(ticketUpdateFlagValues.expirationTimeInput)
		if err == nil {
			ticketUpdateFlagValues.ExpirationTime = exp
		} else {
			ticketUpdateFlagValues.ExpirationTime = time.Time{}
		}
	}

	if command.Flags().Changed("expiry") || command.Flags().Changed("clear_expiry") {
		ticketUpdateFlagValues.ExpirationTimeUpdated = true
	}

	return &ticketUpdateFlagValues
}

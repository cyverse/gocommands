package subcmd

import (
	"strconv"
	"strings"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var modticketCmd = &cobra.Command{
	Use:   "modticket [ticket_name]",
	Short: "Modify a ticket",
	Long:  `This modifies a ticket.`,
	RunE:  processModticketCommand,
}

func AddModticketCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(modticketCmd)

	modticketCmd.Flags().Int64("ulimit", 0, "Set uses limit")
	modticketCmd.Flags().Bool("clear_ulimit", false, "Clear uses limit")
	modticketCmd.Flags().Int64("wflimit", 0, "Set write file limit")
	modticketCmd.Flags().Bool("clear_wflimit", false, "Clear write file limit")
	modticketCmd.Flags().Int64("wblimit", 0, "Set write byte limit")
	modticketCmd.Flags().Bool("clear_wblimit", false, "Clear write byte limit")
	modticketCmd.Flags().String("expiry", "0", "Set expiration time [YYYY:MM:DD HH:mm:SS]")
	modticketCmd.Flags().Bool("clear_expiry", false, "Clear expiration time")
	modticketCmd.Flags().String("add_users", "", "Add allowed users in comma-separated string")
	modticketCmd.Flags().String("rm_users", "", "Remove allowed users in comma-separated string")
	modticketCmd.Flags().String("add_groups", "", "Add allowed groups in comma-separated string")
	modticketCmd.Flags().String("rm_groups", "", "Remove allowed groups in comma-separated string")
	modticketCmd.Flags().String("add_hosts", "", "Add allowed hosts in comma-separated string")
	modticketCmd.Flags().String("rm_hosts", "", "Remove allowed hosts in comma-separated string")

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

	ulimit := int64(0)
	ulimitFlag := command.Flags().Lookup("ulimit")
	if ulimitFlag != nil {
		n, err := strconv.ParseInt(ulimitFlag.Value.String(), 10, 64)
		if err == nil {
			ulimit = n
		}
	}

	clearUlimit := false
	clearUlimitFlag := command.Flags().Lookup("clear_ulimit")
	if clearUlimitFlag != nil {
		clearUlimit, err = strconv.ParseBool(clearUlimitFlag.Value.String())
		if err != nil {
			clearUlimit = false
		}
	}

	wflimit := int64(0)
	wflimitFlag := command.Flags().Lookup("wflimit")
	if wflimitFlag != nil {
		n, err := strconv.ParseInt(wflimitFlag.Value.String(), 10, 64)
		if err == nil {
			wflimit = n
		}
	}

	clearWflimit := false
	clearWflimitFlag := command.Flags().Lookup("clear_wflimit")
	if clearWflimitFlag != nil {
		clearWflimit, err = strconv.ParseBool(clearWflimitFlag.Value.String())
		if err != nil {
			clearWflimit = false
		}
	}

	wblimit := int64(0)
	wblimitFlag := command.Flags().Lookup("wblimit")
	if wblimitFlag != nil {
		n, err := strconv.ParseInt(wblimitFlag.Value.String(), 10, 64)
		if err == nil {
			wblimit = n
		}
	}

	clearWblimit := false
	clearWblimitFlag := command.Flags().Lookup("clear_wblimit")
	if clearWblimitFlag != nil {
		clearWblimit, err = strconv.ParseBool(clearWblimitFlag.Value.String())
		if err != nil {
			clearWblimit = false
		}
	}

	expiry := time.Time{}
	expiryFlag := command.Flags().Lookup("expiry")
	if expiryFlag != nil {
		exp, err := commons.MakeDateTimeFromString(expiryFlag.Value.String())
		if err == nil {
			expiry = exp
		}
	}

	clearExpiry := false
	clearExpiryFlag := command.Flags().Lookup("clear_expiry")
	if clearExpiryFlag != nil {
		clearExpiry, err = strconv.ParseBool(clearExpiryFlag.Value.String())
		if err != nil {
			clearExpiry = false
		}
	}

	addUsers := []string{}
	addUsersFlag := command.Flags().Lookup("add_users")
	if addUsersFlag != nil {
		u := addUsersFlag.Value.String()
		if len(u) > 0 {
			addUsers = strings.Split(u, ",")
		}
	}

	rmUsers := []string{}
	rmUsersFlag := command.Flags().Lookup("rm_users")
	if rmUsersFlag != nil {
		u := rmUsersFlag.Value.String()
		if len(u) > 0 {
			rmUsers = strings.Split(u, ",")
		}

	}

	addGroups := []string{}
	addGroupsFlag := command.Flags().Lookup("add_groups")
	if addGroupsFlag != nil {
		g := addGroupsFlag.Value.String()
		if len(g) > 0 {
			addGroups = strings.Split(g, ",")
		}
	}

	rmGroups := []string{}
	rmGroupsFlag := command.Flags().Lookup("rm_groups")
	if rmGroupsFlag != nil {
		g := rmGroupsFlag.Value.String()
		if len(g) > 0 {
			rmGroups = strings.Split(g, ",")
		}
	}

	addHosts := []string{}
	addHostsFlag := command.Flags().Lookup("add_hosts")
	if addHostsFlag != nil {
		h := addHostsFlag.Value.String()
		if len(h) > 0 {
			addHosts = strings.Split(h, ",")
		}
	}

	rmHosts := []string{}
	rmHostsFlag := command.Flags().Lookup("rm_hosts")
	if rmHostsFlag != nil {
		h := rmHostsFlag.Value.String()
		if len(h) > 0 {
			rmHosts = strings.Split(h, ",")
		}
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

	err = modTicket(filesystem, args[0], ulimit, clearUlimit, wflimit, clearWflimit, wblimit, clearWblimit, expiry, clearExpiry, addUsers, rmUsers, addGroups, rmGroups, addHosts, rmHosts)
	if err != nil {
		return xerrors.Errorf("failed to perform mod ticket for %s: %w", args[0], err)
	}
	return nil
}

func modTicket(fs *irodsclient_fs.FileSystem, ticketName string, ulimit int64, clearUlimit bool, wflimit int64, clearWflimit bool, wblimit int64, clearWblimit bool, expiry time.Time, clearExpiry bool, addUsers []string, rmUsers []string, addGroups []string, rmGroups []string, addHosts []string, rmHosts []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "modTicket",
	})

	logger.Debugf("mod ticket: %s", ticketName)

	if clearUlimit {
		err := fs.ClearTicketUseLimit(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (clear uses limit) %s: %w", ticketName, err)
		}
	} else if ulimit >= 0 {
		err := fs.ModifyTicketUseLimit(ticketName, ulimit)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (modify uses limit) %s: %w", ticketName, err)
		}
	}

	if clearWflimit {
		err := fs.ClearTicketWriteFileLimit(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (clear write file limit) %s: %w", ticketName, err)
		}
	} else if wflimit >= 0 {
		err := fs.ModifyTicketWriteFileLimit(ticketName, wflimit)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (modify write file limit) %s: %w", ticketName, err)
		}
	}

	if clearWblimit {
		err := fs.ClearTicketWriteByteLimit(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (clear write byte limit) %s: %w", ticketName, err)
		}
	} else if wblimit >= 0 {
		err := fs.ModifyTicketWriteByteLimit(ticketName, wblimit)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (modify write byte limit) %s: %w", ticketName, err)
		}
	}

	if clearExpiry {
		err := fs.ClearTicketExpirationTime(ticketName)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (clear expiration time) %s: %w", ticketName, err)
		}
	} else if !expiry.IsZero() {
		err := fs.ModifyTicketExpirationTime(ticketName, expiry)
		if err != nil {
			return xerrors.Errorf("failed to mod ticket (modify expiration time) %s: %w", ticketName, err)
		}
	}

	if len(addUsers) > 0 {
		for _, addUser := range addUsers {
			err := fs.AddTicketAllowedUser(ticketName, addUser)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (add allowed user) %s: %w", ticketName, err)
			}
		}
	}

	if len(rmUsers) > 0 {
		for _, rmUser := range rmUsers {
			err := fs.RemoveTicketAllowedUser(ticketName, rmUser)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (remove allowed user) %s: %w", ticketName, err)
			}
		}
	}

	if len(addGroups) > 0 {
		for _, addGroup := range addGroups {
			err := fs.AddTicketAllowedUser(ticketName, addGroup)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (add allowed group) %s: %w", ticketName, err)
			}
		}
	}

	if len(rmGroups) > 0 {
		for _, rmGroup := range rmGroups {
			err := fs.RemoveTicketAllowedUser(ticketName, rmGroup)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (remove allowed group) %s: %w", ticketName, err)
			}
		}
	}

	if len(addHosts) > 0 {
		for _, addHost := range addHosts {
			err := fs.AddTicketAllowedHost(ticketName, addHost)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (add allowed host) %s: %w", ticketName, err)
			}
		}
	}

	if len(rmHosts) > 0 {
		for _, rmHost := range rmHosts {
			err := fs.RemoveTicketAllowedHost(ticketName, rmHost)
			if err != nil {
				return xerrors.Errorf("failed to mod ticket (remove allowed host) %s: %w", ticketName, err)
			}
		}
	}
	return nil
}

package subcmd

import (
	"fmt"
	"os"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List processes",
	Long:  `This lists processes for iRODS connections establisted in iRODS server.`,
	RunE:  processPsCommand,
}

func AddPsCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(psCmd)

	psCmd.Flags().Bool("groupbyuser", false, "Group processes by user")
	psCmd.Flags().Bool("groupbyprog", false, "Group processes by client program")
	psCmd.Flags().String("zone", "", "Filter by zone")
	psCmd.Flags().String("address", "", "Filter by address")

	rootCmd.AddCommand(psCmd)
}

func processPsCommand(command *cobra.Command, args []string) error {
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

	address := ""
	addressFlag := command.Flags().Lookup("address")
	if addressFlag != nil {
		address = addressFlag.Value.String()
	}

	zone := ""
	zoneFlag := command.Flags().Lookup("zone")
	if zoneFlag != nil {
		zone = zoneFlag.Value.String()
	}

	groupbyuser := false
	groupbyuserFlag := command.Flags().Lookup("groupbyuser")
	if groupbyuserFlag != nil {
		groupbyuser, err = strconv.ParseBool(groupbyuserFlag.Value.String())
		if err != nil {
			groupbyuser = false
		}
	}

	groupbyprog := false
	groupbyprogFlag := command.Flags().Lookup("groupbyprog")
	if groupbyprogFlag != nil {
		groupbyprog, err = strconv.ParseBool(groupbyprogFlag.Value.String())
		if err != nil {
			groupbyprog = false
		}
	}

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	err = listProcesses(filesystem, address, zone, groupbyuser, groupbyprog)
	if err != nil {
		return xerrors.Errorf("failed to perform list processes addr %s, zone %s : %w", address, zone, err)
	}

	return nil
}

func listProcesses(fs *irodsclient_fs.FileSystem, address string, zone string, groupbyuser bool, groupbyprog bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "listProcesses",
	})

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	logger.Debugf("listing processes - addr: %s, zone: %s", address, zone)

	processes, err := irodsclient_irodsfs.StatProcess(connection, address, zone)
	if err != nil {
		return xerrors.Errorf("failed to stat process addr %s, zone %s: %w", address, zone, err)
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if !groupbyprog && !groupbyuser {
		t.AppendHeader(table.Row{
			"Process ID",
			"Proxy User",
			"Client User",
			"Client Address",
			"Client Program",
			"Server Address",
			"Start Time",
		}, table.RowConfig{})

		for _, process := range processes {
			t.AppendRow(table.Row{
				fmt.Sprintf("%d", process.ID),
				fmt.Sprintf("%s#%s", process.ProxyUser, process.ProxyZone),
				fmt.Sprintf("%s#%s", process.ClientUser, process.ClientZone),
				process.ClientAddress,
				process.ClientProgram,
				process.ServerAddress,
				process.StartTime,
			}, table.RowConfig{})
		}
	} else if groupbyuser {
		t.AppendHeader(table.Row{
			"Proxy User",
			"Client User",
			"Process Count",
		}, table.RowConfig{})

		procCount := map[string]int{}
		for _, process := range processes {
			key := fmt.Sprintf("%s#%s,%s#%s", process.ProxyUser, process.ProxyZone, process.ClientUser, process.ClientZone)
			if cnt, ok := procCount[key]; ok {
				// existing
				procCount[key] = cnt + 1
			} else {
				procCount[key] = 1
			}
		}

		procDisplayed := map[string]bool{}
		for _, process := range processes {
			key := fmt.Sprintf("%s#%s,%s#%s", process.ProxyUser, process.ProxyZone, process.ClientUser, process.ClientZone)
			if _, ok := procDisplayed[key]; !ok {
				procDisplayed[key] = true

				t.AppendRow(table.Row{
					fmt.Sprintf("%s#%s", process.ProxyUser, process.ProxyZone),
					fmt.Sprintf("%s#%s", process.ClientUser, process.ClientZone),
					fmt.Sprintf("%d", procCount[key]),
				}, table.RowConfig{})
			}
		}
	} else if groupbyprog {
		t.AppendHeader(table.Row{
			"Client Program",
			"Process Count",
		}, table.RowConfig{})

		procCount := map[string]int{}
		for _, process := range processes {
			key := process.ClientProgram
			if cnt, ok := procCount[key]; ok {
				// existing
				procCount[key] = cnt + 1
			} else {
				procCount[key] = 1
			}
		}

		procDisplayed := map[string]bool{}
		for _, process := range processes {
			key := process.ClientProgram
			if _, ok := procDisplayed[key]; !ok {
				procDisplayed[key] = true

				t.AppendRow(table.Row{
					process.ClientProgram,
					fmt.Sprintf("%d", procCount[key]),
				}, table.RowConfig{})
			}
		}
	}

	t.Render()

	return nil
}

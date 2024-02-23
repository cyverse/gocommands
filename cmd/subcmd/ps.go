package subcmd

import (
	"fmt"
	"os"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"ips"},
	Short:   "List processes",
	Long:    `This lists processes for iRODS connections establisted in iRODS server.`,
	RunE:    processPsCommand,
	Args:    cobra.NoArgs,
}

func AddPsCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(psCmd)

	flag.SetProcessFilterFlags(psCmd)

	rootCmd.AddCommand(psCmd)
}

func processPsCommand(command *cobra.Command, args []string) error {
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

	processFilterFlagValues := flag.GetProcessFilterFlagValues()

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	err = listProcesses(filesystem, processFilterFlagValues)
	if err != nil {
		return xerrors.Errorf("failed to perform list processes addr %s, zone %s : %w", processFilterFlagValues.Address, processFilterFlagValues.Zone, err)
	}

	return nil
}

func listProcesses(fs *irodsclient_fs.FileSystem, processFilterFlagValues *flag.ProcessFilterFlagValues) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "listProcesses",
	})

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	logger.Debugf("listing processes - addr: %s, zone: %s", processFilterFlagValues.Address, processFilterFlagValues.Zone)

	processes, err := irodsclient_irodsfs.StatProcess(connection, processFilterFlagValues.Address, processFilterFlagValues.Zone)
	if err != nil {
		return xerrors.Errorf("failed to stat process addr %s, zone %s: %w", processFilterFlagValues.Address, processFilterFlagValues.Zone, err)
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	switch processFilterFlagValues.GroupBy {
	case flag.ProcessGroupByNone:
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
	case flag.ProcessGroupByUser:
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
	case flag.ProcessGroupByProgram:
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

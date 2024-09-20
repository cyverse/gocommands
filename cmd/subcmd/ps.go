package subcmd

import (
	"fmt"
	"os"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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
	flag.SetCommonFlags(psCmd, true)

	flag.SetProcessFilterFlags(psCmd)

	rootCmd.AddCommand(psCmd)
}

func processPsCommand(command *cobra.Command, args []string) error {
	ps, err := NewPsCommand(command, args)
	if err != nil {
		return err
	}

	return ps.Process()
}

type PsCommand struct {
	command *cobra.Command

	processFilterFlagValues *flag.ProcessFilterFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewPsCommand(command *cobra.Command, args []string) (*PsCommand, error) {
	ps := &PsCommand{
		command: command,

		processFilterFlagValues: flag.GetProcessFilterFlagValues(),
	}

	return ps, nil
}

func (ps *PsCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(ps.command)
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

	// Create a connection
	ps.account = commons.GetSessionConfig().ToIRODSAccount()
	ps.filesystem, err = commons.GetIRODSFSClient(ps.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer ps.filesystem.Release()

	err = ps.listProcesses()
	if err != nil {
		return xerrors.Errorf("failed to list processes: %w", err)
	}

	return nil
}

func (ps *PsCommand) listProcesses() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PsCommand",
		"function": "listProcesses",
	})

	connection, err := ps.filesystem.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer ps.filesystem.ReturnMetadataConnection(connection)

	logger.Debugf("listing processes - addr: %q, zone: %q", ps.processFilterFlagValues.Address, ps.processFilterFlagValues.Zone)

	processes, err := irodsclient_irodsfs.StatProcess(connection, ps.processFilterFlagValues.Address, ps.processFilterFlagValues.Zone)
	if err != nil {
		return xerrors.Errorf("failed to stat process addr %q, zone %q: %w", ps.processFilterFlagValues.Address, ps.processFilterFlagValues.Zone, err)
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	switch ps.processFilterFlagValues.GroupBy {
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

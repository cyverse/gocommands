package subcmd

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"ips"},
	Short:   "List iRODS processes",
	Long:    `This command lists the processes for iRODS connections established on the iRODS server.`,
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

	commonFlagValues        *flag.CommonFlagValues
	processFilterFlagValues *flag.ProcessFilterFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewPsCommand(command *cobra.Command, args []string) (*PsCommand, error) {
	ps := &PsCommand{
		command: command,

		commonFlagValues:        flag.GetCommonFlagValues(command),
		processFilterFlagValues: flag.GetProcessFilterFlagValues(),
	}

	return ps, nil
}

func (ps *PsCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(ps.command)
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

	// Create a connection
	ps.account = config.GetSessionConfig().ToIRODSAccount()
	ps.filesystem, err = irods.GetIRODSFSClient(ps.account, true, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer ps.filesystem.Release()

	if ps.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(ps.filesystem, ps.commonFlagValues.Timeout)
	}

	err = ps.listProcesses()
	if err != nil {
		return errors.Wrapf(err, "failed to list processes")
	}

	return nil
}

func (ps *PsCommand) listProcesses() error {
	logger := log.WithFields(log.Fields{
		"address": ps.processFilterFlagValues.Address,
		"zone":    ps.processFilterFlagValues.Zone,
	})

	logger.Debug("listing processes")

	processes, err := ps.filesystem.StatProcess(ps.processFilterFlagValues.Address, ps.processFilterFlagValues.Zone)
	if err != nil {
		return errors.Wrapf(err, "failed to stat process addr %q, zone %q", ps.processFilterFlagValues.Address, ps.processFilterFlagValues.Zone)
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

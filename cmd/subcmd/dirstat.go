package subcmd

import (
	"fmt"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var dirstatCmd = &cobra.Command{
	Use:     "dirstat <irods-object>...",
	Aliases: []string{"dir_stat", "dir_statistics"},
	Short:   "Display statistics for iRODS directories",
	Long:    `This command displays statistics for a specified iRODS directory, including total size and file count.`,
	RunE:    processDirstatCommand,
	Args:    cobra.ArbitraryArgs,
}

func AddDirstatCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(dirstatCmd, false)

	flag.SetRecursiveFlags(dirstatCmd, false)
	flag.SetOutputFormatFlags(dirstatCmd, true)
	flag.SetListFlagsForHumanReadableSizes(dirstatCmd)

	rootCmd.AddCommand(dirstatCmd)
}

func processDirstatCommand(command *cobra.Command, args []string) error {
	dirStat, err := NewDirStatCommand(command, args)
	if err != nil {
		return err
	}

	return dirStat.Process()
}

type DirStatCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	recursiveFlagValues    *flag.RecursiveFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
	listFlagValues         *flag.ListFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
}

func NewDirStatCommand(command *cobra.Command, args []string) (*DirStatCommand, error) {
	dirStat := &DirStatCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		recursiveFlagValues:    flag.GetRecursiveFlagValues(),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
		listFlagValues:         flag.GetListFlagValues(),
	}

	// path
	dirStat.sourcePaths = args[:]

	if len(args) == 0 {
		dirStat.sourcePaths = []string{"."}
	}

	return dirStat, nil
}

func (dirStat *DirStatCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(dirStat.command)
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
	dirStat.account = config.GetSessionConfig().ToIRODSAccount()

	timeout := 0
	if dirStat.commonFlagValues.TimeoutUpdated {
		timeout = dirStat.commonFlagValues.Timeout
	}

	dirStat.filesystem, err = irods.GetIRODSFSClient(dirStat.account, true, timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer dirStat.filesystem.Release()

	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())
	outputFormatterTable := outputFormatter.NewTable("iRODS Collection Statistics")

	if len(dirStat.sourcePaths) == 0 {
		return errors.New("no target objects specified")
	}

	columns := []string{
		"Path",
		"Total Data Objects",
		"Total Size",
	}

	outputFormatterTable.SetHeader(columns)

	// run
	for _, targetObject := range dirStat.sourcePaths {
		err = dirStat.getDirStat(outputFormatterTable, targetObject)
		if err != nil {
			return err
		}
	}

	if dirStat.outputFormatFlagValues.Format == format.OutputFormatLegacy {
		dirStat.outputFormatFlagValues.Format = format.OutputFormatTable
	}
	outputFormatter.Render(dirStat.outputFormatFlagValues.Format)

	return nil
}

func (dirStat *DirStatCommand) getDirStat(outputFormatterTable *format.OutputFormatterTable, sourcePath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := dirStat.account.ClientZone
	sourcePath = path.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := dirStat.filesystem.Stat(sourcePath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return errors.Wrapf(err, "failed to find collection %q", sourcePath)
		}

		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if !sourceEntry.IsDir() {
		// file
		return errors.Errorf("failed to list data-object %q, must be a directory", sourcePath)
	}

	stat, err := dirStat.filesystem.GetDirStatistics(sourcePath, dirStat.recursiveFlagValues.Recursive)
	if err != nil {
		return errors.Wrapf(err, "failed to get directory statistics for path %q", sourcePath)
	}

	return dirStat.printDirStats(outputFormatterTable, stat)
}

func (dirStat *DirStatCommand) printDirStats(outputFormatterTable *format.OutputFormatterTable, stat *irodsclient_fs.DirStat) error {
	size := fmt.Sprintf("%v", stat.TotalSize)
	if dirStat.listFlagValues.HumanReadableSizes {
		size = humanize.Bytes(uint64(stat.TotalSize))
	}

	columnValues := []interface{}{
		stat.Path,
		stat.FileCount,
		size,
	}

	outputFormatterTable.AppendRow(columnValues)

	return nil
}

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
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var lsmetaCmd = &cobra.Command{
	Use:     "lsmeta <irods-object>...",
	Aliases: []string{"ls_meta", "ls_metadata", "list_meta", "list_metadata"},
	Short:   "List metadata for iRODS collections, data objects, users, or resources",
	Long:    `This command lists metadata associated with a specified iRODS object, such as a collection, data object, user, or resource.`,
	RunE:    processLsmetaCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddLsmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlagsWithoutResource(lsmetaCmd)
	flag.SetOutputFormatFlags(lsmetaCmd)
	flag.SetListFlags(lsmetaCmd, true, true)
	flag.SetTargetObjectFlags(lsmetaCmd)

	rootCmd.AddCommand(lsmetaCmd)
}

func processLsmetaCommand(command *cobra.Command, args []string) error {
	lsMeta, err := NewLsMetaCommand(command, args)
	if err != nil {
		return err
	}

	return lsMeta.Process()
}

type LsMetaCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
	listFlagValues         *flag.ListFlagValues
	targetObjectFlagValues *flag.TargetObjectFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetObjects []string
}

func NewLsMetaCommand(command *cobra.Command, args []string) (*LsMetaCommand, error) {
	lsMeta := &LsMetaCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
		listFlagValues:         flag.GetListFlagValues(),
		targetObjectFlagValues: flag.GetTargetObjectFlagValues(command),
	}

	lsMeta.targetObjects = args[:]

	return lsMeta, nil
}

func (lsMeta *LsMetaCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(lsMeta.command)
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
	lsMeta.account = config.GetSessionConfig().ToIRODSAccount()
	lsMeta.filesystem, err = irods.GetIRODSFSClient(lsMeta.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer lsMeta.filesystem.Release()

	if lsMeta.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(lsMeta.filesystem, lsMeta.commonFlagValues.Timeout)
	}

	// table writer
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(terminal.GetTerminalWriter())
	tableWriter.SetTitle("iRODS Metadata")

	if len(lsMeta.targetObjects) == 0 {
		return errors.New("no target objects specified")
	}

	columns := []interface{}{
		"ID",
		"Attribute",
		"Value",
		"Unit",
	}

	if lsMeta.listFlagValues.Format == format.ListFormatLong || lsMeta.listFlagValues.Format == format.ListFormatVeryLong {
		columns = append(columns,
			"Create Time",
			"Modify Time",
		)
	}

	tableWriter.AppendHeader(columns, table.RowConfig{})

	// run
	for _, targetObject := range lsMeta.targetObjects {
		if lsMeta.targetObjectFlagValues.Path {
			err = lsMeta.listMetaForPath(tableWriter, targetObject)
			if err != nil {
				return err
			}
		} else if lsMeta.targetObjectFlagValues.User {
			err = lsMeta.listMetaForUser(tableWriter, targetObject)
			if err != nil {
				return err
			}
		} else if lsMeta.targetObjectFlagValues.Resource {
			err = lsMeta.listMetaForResource(tableWriter, targetObject)
			if err != nil {
				return err
			}
		}
	}

	switch lsMeta.outputFormatFlagValues.Format {
	case format.OutputFormatCSV:
		tableWriter.RenderCSV()
	case format.OutputFormatTSV:
		tableWriter.RenderTSV()
	default:
		tableWriter.Render()
	}

	return nil
}

func (lsMeta *LsMetaCommand) listMetaForPath(tableWriter table.Writer, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := lsMeta.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	metas, err := lsMeta.filesystem.ListMetadata(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to list meta for path %q", targetPath)
	}

	return lsMeta.printMetas(tableWriter, metas)
}

func (lsMeta *LsMetaCommand) listMetaForUser(tableWriter table.Writer, username string) error {
	metas, err := lsMeta.filesystem.ListUserMetadata(username, lsMeta.account.ClientZone)
	if err != nil {
		return errors.Wrapf(err, "failed to list meta for user %q", username)
	}

	return lsMeta.printMetas(tableWriter, metas)
}

func (lsMeta *LsMetaCommand) listMetaForResource(tableWriter table.Writer, resource string) error {
	metas, err := lsMeta.filesystem.ListResourceMetadata(resource)
	if err != nil {
		return errors.Wrapf(err, "failed to list meta for resource %q", resource)
	}

	return lsMeta.printMetas(tableWriter, metas)
}

func (lsMeta *LsMetaCommand) printMetas(tableWriter table.Writer, metas []*irodsclient_types.IRODSMeta) error {
	sort.SliceStable(metas, lsMeta.getMetaSortFunction(metas, lsMeta.listFlagValues.SortOrder, lsMeta.listFlagValues.SortReverse))

	for _, meta := range metas {
		lsMeta.printMetaInternal(tableWriter, meta)
	}

	return nil
}

func (lsMeta *LsMetaCommand) printMetaInternal(tableWriter table.Writer, meta *irodsclient_types.IRODSMeta) {
	name := meta.Name
	if len(name) == 0 {
		name = "<empty>"
	}

	value := meta.Value
	if len(value) == 0 {
		value = "<empty>"
	}

	units := meta.Units
	if len(units) == 0 {
		units = "<empty>"
	}

	columnValues := []interface{}{
		meta.AVUID,
		name,
		value,
		units,
	}

	if lsMeta.listFlagValues.Format == format.ListFormatLong || lsMeta.listFlagValues.Format == format.ListFormatVeryLong {
		createTime := types.MakeDateTimeString(meta.CreateTime)
		modTime := types.MakeDateTimeString(meta.ModifyTime)

		columnValues = append(columnValues,
			createTime,
			modTime,
		)
	}

	tableWriter.AppendRow(columnValues, table.RowConfig{})
}

func (lsMeta *LsMetaCommand) getMetaSortFunction(metas []*irodsclient_types.IRODSMeta, sortOrder format.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case format.ListSortOrderName:
			return func(i int, j int) bool {
				return metas[i].Name > metas[j].Name
			}
		case format.ListSortOrderTime:
			return func(i int, j int) bool {
				return (metas[i].ModifyTime.After(metas[j].ModifyTime)) ||
					(metas[i].ModifyTime.Equal(metas[j].ModifyTime) &&
						metas[i].Name < metas[j].Name)
			}
		// Cannot sort meta by size or extension, so use default sort by avuid
		default:
			return func(i int, j int) bool {
				return metas[i].AVUID < metas[j].AVUID
			}
		}
	}

	switch sortOrder {
	case format.ListSortOrderName:
		return func(i int, j int) bool {
			return metas[i].Name < metas[j].Name
		}
	case format.ListSortOrderTime:
		return func(i int, j int) bool {
			return (metas[i].ModifyTime.Before(metas[j].ModifyTime)) ||
				(metas[i].ModifyTime.Equal(metas[j].ModifyTime) &&
					metas[i].Name < metas[j].Name)

		}
		// Cannot sort meta by size or extension, so use default sort by avuid
	default:
		return func(i int, j int) bool {
			return metas[i].AVUID < metas[j].AVUID
		}
	}
}

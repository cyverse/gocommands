package subcmd

import (
	"fmt"
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var lsmetaCmd = &cobra.Command{
	Use:     "lsmeta",
	Aliases: []string{"ls_meta", "ls_metadata", "list_meta", "list_metadata"},
	Short:   "List metadata",
	Long:    `This lists metadata for the given collection, data object, user, or a resource.`,
	RunE:    processLsmetaCommand,
	Args:    cobra.NoArgs,
}

func AddLsmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlagsWithoutResource(lsmetaCmd)

	flag.SetListFlags(lsmetaCmd)
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
	listFlagValues         *flag.ListFlagValues
	targetObjectFlagValues *flag.TargetObjectFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewLsMetaCommand(command *cobra.Command, args []string) (*LsMetaCommand, error) {
	lsMeta := &LsMetaCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		listFlagValues:         flag.GetListFlagValues(),
		targetObjectFlagValues: flag.GetTargetObjectFlagValues(command),
	}

	return lsMeta, nil
}

func (lsMeta *LsMetaCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(lsMeta.command)
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
	lsMeta.account = commons.GetSessionConfig().ToIRODSAccount()
	lsMeta.filesystem, err = commons.GetIRODSFSClientForSingleOperation(lsMeta.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer lsMeta.filesystem.Release()

	if lsMeta.targetObjectFlagValues.PathUpdated {
		return lsMeta.listMetaForPath(lsMeta.targetObjectFlagValues.Path)
	} else if lsMeta.targetObjectFlagValues.UserUpdated {
		return lsMeta.listMetaForUser(lsMeta.targetObjectFlagValues.User)
	} else if lsMeta.targetObjectFlagValues.ResourceUpdated {
		return lsMeta.listMetaForResource(lsMeta.targetObjectFlagValues.Resource)
	}

	// nothing updated
	return xerrors.Errorf("path, user, or resource must be given")
}

func (lsMeta *LsMetaCommand) listMetaForPath(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := lsMeta.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	metas, err := lsMeta.filesystem.ListMetadata(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to list meta for path %q: %w", targetPath, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
		return nil
	}

	return lsMeta.printMetas(metas)
}

func (lsMeta *LsMetaCommand) listMetaForUser(username string) error {
	metas, err := lsMeta.filesystem.ListUserMetadata(username, lsMeta.account.ClientZone)
	if err != nil {
		return xerrors.Errorf("failed to list meta for user %q: %w", username, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
		return nil
	}

	return lsMeta.printMetas(metas)
}

func (lsMeta *LsMetaCommand) listMetaForResource(resource string) error {
	metas, err := lsMeta.filesystem.ListResourceMetadata(resource)
	if err != nil {
		return xerrors.Errorf("failed to list meta for resource %q: %w", resource, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
		return nil
	}

	return lsMeta.printMetas(metas)
}

func (lsMeta *LsMetaCommand) printMetas(metas []*irodsclient_types.IRODSMeta) error {
	sort.SliceStable(metas, lsMeta.getMetaSortFunction(metas, lsMeta.listFlagValues.SortOrder, lsMeta.listFlagValues.SortReverse))

	for _, meta := range metas {
		lsMeta.printMetaInternal(meta)
	}

	return nil
}

func (lsMeta *LsMetaCommand) printMetaInternal(meta *irodsclient_types.IRODSMeta) {
	createTime := commons.MakeDateTimeString(meta.CreateTime)
	modTime := commons.MakeDateTimeString(meta.ModifyTime)

	name := meta.Name
	if len(name) == 0 {
		name = "<empty name>"
	} else {
		name = fmt.Sprintf("\"%s\"", name)
	}

	value := meta.Value
	if len(value) == 0 {
		value = "<empty value>"
	} else {
		value = fmt.Sprintf("\"%s\"", value)
	}

	units := meta.Units
	if len(units) == 0 {
		units = "<empty units>"
	} else {
		units = fmt.Sprintf("\"%s\"", units)
	}

	switch lsMeta.listFlagValues.Format {
	case commons.ListFormatLong, commons.ListFormatVeryLong:
		commons.Printf("[%s]\n", meta.Name)
		commons.Printf("[%s]\n", meta.Name)
		commons.Printf("  id: %d\n", meta.AVUID)
		commons.Printf("  attribute: %s\n", name)
		commons.Printf("  value: %s\n", value)
		commons.Printf("  unit: %s\n", units)
		commons.Printf("  create time: %s\n", createTime)
		commons.Printf("  modify time: %s\n", modTime)
	case commons.ListFormatNormal:
		fallthrough
	default:
		commons.Printf("%d\t%s\t%s\t%s\n", meta.AVUID, name, value, units)
	}
}

func (lsMeta *LsMetaCommand) getMetaSortFunction(metas []*irodsclient_types.IRODSMeta, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case commons.ListSortOrderName:
			return func(i int, j int) bool {
				return metas[i].Name > metas[j].Name
			}
		case commons.ListSortOrderTime:
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
	case commons.ListSortOrderName:
		return func(i int, j int) bool {
			return metas[i].Name < metas[j].Name
		}
	case commons.ListSortOrderTime:
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

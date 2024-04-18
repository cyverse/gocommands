package subcmd

import (
	"fmt"
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
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
	flag.SetCommonFlags(lsmetaCmd, true)

	flag.SetListFlags(lsmetaCmd)
	flag.SetTargetObjectFlags(lsmetaCmd)

	rootCmd.AddCommand(lsmetaCmd)
}

func processLsmetaCommand(command *cobra.Command, args []string) error {
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

	listFlagValues := flag.GetListFlagValues()
	targetObjectFlagValues := flag.GetTargetObjectFlagValues(command)

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if targetObjectFlagValues.PathUpdated {
		err = listMetaForPath(filesystem, targetObjectFlagValues.Path, listFlagValues)
		if err != nil {
			return err
		}
	} else if targetObjectFlagValues.UserUpdated {
		err = listMetaForUser(filesystem, targetObjectFlagValues.User, listFlagValues)
		if err != nil {
			return err
		}
	} else if targetObjectFlagValues.ResourceUpdated {
		err = listMetaForResource(filesystem, targetObjectFlagValues.Resource, listFlagValues)
		if err != nil {
			return err
		}
	} else {
		// nothing updated
		return xerrors.Errorf("path, user, or resource must be given")
	}

	return nil
}

func listMetaForPath(fs *irodsclient_fs.FileSystem, targetPath string, listFlagValues *flag.ListFlagValues) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	metas, err := fs.ListMetadata(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to list meta for path %s: %w", targetPath, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
	} else {
		err = printMetas(metas, listFlagValues)
		if err != nil {
			return err
		}
	}

	return nil
}

func listMetaForUser(fs *irodsclient_fs.FileSystem, username string, listFlagValues *flag.ListFlagValues) error {
	metas, err := fs.ListUserMetadata(username)
	if err != nil {
		return xerrors.Errorf("failed to list meta for user %s: %w", username, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
	} else {
		err = printMetas(metas, listFlagValues)
		if err != nil {
			return err
		}
	}

	return nil
}

func listMetaForResource(fs *irodsclient_fs.FileSystem, resource string, listFlagValues *flag.ListFlagValues) error {
	metas, err := fs.ListResourceMetadata(resource)
	if err != nil {
		return xerrors.Errorf("failed to list meta for resource %s: %w", resource, err)
	}

	if len(metas) == 0 {
		commons.Printf("Found no metadata\n")
	} else {
		err = printMetas(metas, listFlagValues)
		if err != nil {
			return err
		}
	}

	return nil
}

func getMetaSortFunction(metas []*types.IRODSMeta, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
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

func printMetas(metas []*types.IRODSMeta, listFlagValues *flag.ListFlagValues) error {
	sort.SliceStable(metas, getMetaSortFunction(metas, listFlagValues.SortOrder, listFlagValues.SortReverse))

	for _, meta := range metas {
		printMetaInternal(meta, listFlagValues)
	}

	return nil
}

func printMetaInternal(meta *types.IRODSMeta, listFlagValues *flag.ListFlagValues) {
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

	switch listFlagValues.Format {
	case commons.ListFormatLong, commons.ListFormatVeryLong:
		fmt.Printf("[%s]\n", meta.Name)
		fmt.Printf("[%s]\n", meta.Name)
		fmt.Printf("  id: %d\n", meta.AVUID)
		fmt.Printf("  attribute: %s\n", name)
		fmt.Printf("  value: %s\n", value)
		fmt.Printf("  unit: %s\n", units)
		fmt.Printf("  create time: %s\n", createTime)
		fmt.Printf("  modify time: %s\n", modTime)
	case commons.ListFormatNormal:
		fallthrough
	default:
		fmt.Printf("%d\t%s\t%s\t%s\n", meta.AVUID, name, value, units)
	}
}

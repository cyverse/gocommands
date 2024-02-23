package subcmd

import (
	"fmt"
	"path"
	"sort"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

/*
A struct containing a replica and its data object. Used so that we can easily sort

	replicas by either replica properties or data object properties.
*/
type FlatReplica struct {
	Replica    *irodsclient_types.IRODSReplica
	DataObject *irodsclient_types.IRODSDataObject
}

var lsCmd = &cobra.Command{
	Use:     "ls [collection1] [collection2] ...",
	Aliases: []string{"ils", "list"},
	Short:   "List entries in iRODS collections",
	Long:    `This lists data objects and collections in iRODS collections.`,
	RunE:    processLsCommand,
	Args:    cobra.ArbitraryArgs,
}

func AddLsCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(lsCmd)

	flag.SetListFlags(lsCmd)
	flag.SetTicketAccessFlags(lsCmd)

	rootCmd.AddCommand(lsCmd)
}

func processLsCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processLsCommand",
	})

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

	ticketAccessFlagValues := flag.GetTicketAccessFlagValues()
	listFlagValues := flag.GetListFlagValues()

	appConfig := commons.GetConfig()
	syncAccount := false
	if len(ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %s", ticketAccessFlagValues.Name)
		appConfig.Ticket = ticketAccessFlagValues.Name
		syncAccount = true
	}

	if syncAccount {
		err := commons.SyncAccount()
		if err != nil {
			return err
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	sourcePaths := args[:]

	if len(args) == 0 {
		sourcePaths = []string{"."}
	}

	for _, sourcePath := range sourcePaths {
		err = listOne(filesystem, sourcePath, listFlagValues)
		if err != nil {
			return xerrors.Errorf("failed to perform ls %s: %w", sourcePath, err)
		}
	}

	return nil
}

func listOne(fs *irodsclient_fs.FileSystem, sourcePath string, listFlagValues *flag.ListFlagValues) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	collection, err := irodsclient_irodsfs.GetCollection(connection, sourcePath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to get collection %s: %w", sourcePath, err)
		}
	}

	if err == nil {
		colls, err := irodsclient_irodsfs.ListSubCollections(connection, sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to list sub-collections in %s: %w", sourcePath, err)
		}

		objs, err := irodsclient_irodsfs.ListDataObjects(connection, collection)
		if err != nil {
			return xerrors.Errorf("failed to list data-objects in %s: %w", sourcePath, err)
		}

		printDataObjects(objs, listFlagValues.Format, listFlagValues.HumanReadableSizes, listFlagValues.SortOrder, listFlagValues.SortReverse)
		printCollections(colls, listFlagValues.SortOrder, listFlagValues.SortReverse)
		return nil
	}

	// data object
	parentSourcePath := path.Dir(sourcePath)

	parentCollection, err := irodsclient_irodsfs.GetCollection(connection, parentSourcePath)
	if err != nil {
		return xerrors.Errorf("failed to get collection %s: %w", parentSourcePath, err)
	}

	entry, err := irodsclient_irodsfs.GetDataObject(connection, parentCollection, path.Base(sourcePath))
	if err != nil {
		return xerrors.Errorf("failed to get data-object %s: %w", sourcePath, err)
	}

	entries := []*irodsclient_types.IRODSDataObject{entry}
	printDataObjects(entries, listFlagValues.Format, listFlagValues.HumanReadableSizes, listFlagValues.SortOrder, listFlagValues.SortReverse)
	return nil
}

func flattenReplicas(objects []*irodsclient_types.IRODSDataObject) []*FlatReplica {
	var result []*FlatReplica
	for _, object := range objects {
		for _, replica := range object.Replicas {
			flatReplica := FlatReplica{DataObject: object, Replica: replica}
			result = append(result, &flatReplica)
		}
	}
	return result
}

func getFlatReplicaSortFunction(entries []*FlatReplica, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case commons.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].DataObject.Name > entries[j].DataObject.Name
			}
		case commons.ListSortOrderExt:
			return func(i int, j int) bool {
				return (path.Ext(entries[i].DataObject.Name) > path.Ext(entries[j].DataObject.Name)) ||
					(path.Ext(entries[i].DataObject.Name) == path.Ext(entries[j].DataObject.Name) &&
						entries[i].DataObject.Name < entries[j].DataObject.Name)
			}
		case commons.ListSortOrderTime:
			return func(i int, j int) bool {
				return (entries[i].Replica.ModifyTime.After(entries[j].Replica.ModifyTime)) ||
					(entries[i].Replica.ModifyTime.Equal(entries[j].Replica.ModifyTime) &&
						entries[i].DataObject.Name < entries[j].DataObject.Name)
			}
		case commons.ListSortOrderSize:
			return func(i int, j int) bool {
				return (entries[i].DataObject.Size > entries[j].DataObject.Size) ||
					(entries[i].DataObject.Size == entries[j].DataObject.Size &&
						entries[i].DataObject.Name < entries[j].DataObject.Name)
			}
		default:
			return func(i int, j int) bool {
				return entries[i].DataObject.Name > entries[j].DataObject.Name
			}
		}
	}

	switch sortOrder {
	case commons.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].DataObject.Name < entries[j].DataObject.Name
		}
	case commons.ListSortOrderExt:
		return func(i int, j int) bool {
			return (path.Ext(entries[i].DataObject.Name) < path.Ext(entries[j].DataObject.Name)) ||
				(path.Ext(entries[i].DataObject.Name) == path.Ext(entries[j].DataObject.Name) &&
					entries[i].DataObject.Name < entries[j].DataObject.Name)
		}
	case commons.ListSortOrderTime:
		return func(i int, j int) bool {
			return (entries[i].Replica.ModifyTime.Before(entries[j].Replica.ModifyTime)) ||
				(entries[i].Replica.ModifyTime.Equal(entries[j].Replica.ModifyTime) &&
					entries[i].DataObject.Name < entries[j].DataObject.Name)
		}
	case commons.ListSortOrderSize:
		return func(i int, j int) bool {
			return (entries[i].DataObject.Size < entries[j].DataObject.Size) ||
				(entries[i].DataObject.Size == entries[j].DataObject.Size &&
					entries[i].DataObject.Name < entries[j].DataObject.Name)
		}
	default:
		return func(i int, j int) bool {
			return entries[i].DataObject.Name < entries[j].DataObject.Name
		}
	}
}

func printDataObjects(entries []*irodsclient_types.IRODSDataObject, format commons.ListFormat, humanReadableSizes bool, sortOrder commons.ListSortOrder, sortReverse bool) {
	if format == commons.ListFormatNormal {
		sort.SliceStable(entries, getDataObjectSortFunction(entries, sortOrder, sortReverse))
		for _, entry := range entries {
			printDataObjectShort(entry)
		}
	} else {
		replicas := flattenReplicas(entries)
		sort.SliceStable(replicas, getFlatReplicaSortFunction(replicas, sortOrder, sortReverse))
		printReplicas(replicas, format, humanReadableSizes)
	}
}

func getDataObjectSortFunction(entries []*irodsclient_types.IRODSDataObject, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case commons.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].Name > entries[j].Name
			}
		case commons.ListSortOrderExt:
			return func(i int, j int) bool {
				return (path.Ext(entries[i].Name) > path.Ext(entries[j].Name)) ||
					(path.Ext(entries[i].Name) == path.Ext(entries[j].Name) &&
						entries[i].Name < entries[j].Name)
			}
		case commons.ListSortOrderTime:
			return func(i int, j int) bool {
				return (getDataObjectModifyTime(entries[i]).After(getDataObjectModifyTime(entries[j]))) ||
					(getDataObjectModifyTime(entries[i]).Equal(getDataObjectModifyTime(entries[j])) &&
						entries[i].Name < entries[j].Name)
			}
		case commons.ListSortOrderSize:
			return func(i int, j int) bool {
				return (entries[i].Size > entries[j].Size) ||
					(entries[i].Size == entries[j].Size &&
						entries[i].Name < entries[j].Name)
			}
		default:
			return func(i int, j int) bool {
				return entries[i].Name > entries[j].Name
			}
		}
	}

	switch sortOrder {
	case commons.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	case commons.ListSortOrderExt:
		return func(i int, j int) bool {
			return (path.Ext(entries[i].Name) < path.Ext(entries[j].Name)) ||
				(path.Ext(entries[i].Name) == path.Ext(entries[j].Name) &&
					entries[i].Name < entries[j].Name)
		}
	case commons.ListSortOrderTime:
		return func(i int, j int) bool {
			return (getDataObjectModifyTime(entries[i]).Before(getDataObjectModifyTime(entries[j]))) ||
				(getDataObjectModifyTime(entries[i]).Equal(getDataObjectModifyTime(entries[j])) &&
					entries[i].Name < entries[j].Name)
		}
	case commons.ListSortOrderSize:
		return func(i int, j int) bool {
			return (entries[i].Size < entries[j].Size) ||
				(entries[i].Size == entries[j].Size &&
					entries[i].Name < entries[j].Name)
		}
	default:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	}
}

func getDataObjectModifyTime(object *irodsclient_types.IRODSDataObject) time.Time {
	// ModifyTime of data object is considered to be ModifyTime of replica modified most recently
	maxTime := object.Replicas[0].ModifyTime
	for _, t := range object.Replicas[1:] {
		if t.ModifyTime.After(maxTime) {
			maxTime = t.ModifyTime
		}
	}
	return maxTime
}

func printDataObjectShort(entry *irodsclient_types.IRODSDataObject) {
	fmt.Printf("  %s\n", entry.Name)
}

func printReplicas(flatReplicas []*FlatReplica, format commons.ListFormat, humanReadableSizes bool) {
	for _, flatReplica := range flatReplicas {
		printReplica(*flatReplica, format, humanReadableSizes)
	}
}

func printReplica(flatReplica FlatReplica, format commons.ListFormat, humanReadableSizes bool) {
	size := fmt.Sprintf("%v", flatReplica.DataObject.Size)
	if humanReadableSizes {
		size = humanize.Bytes(uint64(flatReplica.DataObject.Size))
	}
	switch format {
	case commons.ListFormatNormal:
		fmt.Printf("  %d\t%s\n", flatReplica.Replica.Number, flatReplica.DataObject.Name)
	case commons.ListFormatLong:
		modTime := commons.MakeDateTimeString(flatReplica.Replica.ModifyTime)
		fmt.Printf("  %s\t%d\t%s\t%s\t%s\t%s\t%s\n", flatReplica.Replica.Owner, flatReplica.Replica.Number, flatReplica.Replica.ResourceHierarchy,
			size, modTime, getStatusMark(flatReplica.Replica.Status), flatReplica.DataObject.Name)
	case commons.ListFormatVeryLong:
		modTime := commons.MakeDateTimeString(flatReplica.Replica.ModifyTime)
		fmt.Printf("  %s\t%d\t%s\t%s\t%s\t%s\t%s\n", flatReplica.Replica.Owner, flatReplica.Replica.Number, flatReplica.Replica.ResourceHierarchy,
			size, modTime, getStatusMark(flatReplica.Replica.Status), flatReplica.DataObject.Name)
		fmt.Printf("    %s\t%s\n", flatReplica.Replica.Checksum.OriginalChecksum, flatReplica.Replica.Path)
	default:
		fmt.Printf("  %d\t%s\n", flatReplica.Replica.Number, flatReplica.DataObject.Name)
	}
}

func printCollections(entries []*irodsclient_types.IRODSCollection, sortOrder commons.ListSortOrder, sortReverse bool) {
	sort.SliceStable(entries, getCollectionSortFunction(entries, sortOrder, sortReverse))
	for _, entry := range entries {
		fmt.Printf("  C- %s\n", entry.Path)
	}
}

func getCollectionSortFunction(entries []*irodsclient_types.IRODSCollection, sortOrder commons.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case commons.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].Name > entries[j].Name
			}
		case commons.ListSortOrderTime:
			return func(i int, j int) bool {
				return (entries[i].ModifyTime.After(entries[j].ModifyTime)) ||
					(entries[i].ModifyTime.Equal(entries[j].ModifyTime) &&
						entries[i].Name < entries[j].Name)
			}
		// Cannot sort collections by size or extension, so use default sort by name
		default:
			return func(i int, j int) bool {
				return entries[i].Name < entries[j].Name
			}
		}
	}

	switch sortOrder {
	case commons.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	case commons.ListSortOrderTime:
		return func(i int, j int) bool {
			return (entries[i].ModifyTime.Before(entries[j].ModifyTime)) ||
				(entries[i].ModifyTime.Equal(entries[j].ModifyTime) &&
					entries[i].Name < entries[j].Name)

		}
		// Cannot sort collections by size or extension, so use default sort by name
	default:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	}
}

func getStatusMark(status string) string {
	switch status {
	case "0":
		return "X" // stale
	case "1":
		return "&" // good
	default:
		return "?"
	}
}

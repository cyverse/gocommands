package subcmd

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/encryption"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/irods"
	commons_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/cyverse/gocommands/commons/wildcard"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// FlatReplica is a struct containing a replica and its data object. Used so that we can easily sort
// replicas by either replica properties or data object properties.
type FlatReplica struct {
	Replica    *irodsclient_types.IRODSReplica
	DataObject *irodsclient_types.IRODSDataObject
}

var lsCmd = &cobra.Command{
	Use:     "ls <data-object-or-collection>...",
	Aliases: []string{"ils", "list"},
	Short:   "List data objects or entries in iRODS collections",
	Long:    `This command lists data objects and collections within the specified iRODS collections.`,
	RunE:    processLsCommand,
	Args:    cobra.ArbitraryArgs,
}

func AddLsCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(lsCmd, false)
	flag.SetOutputFormatFlags(lsCmd)
	flag.SetListFlags(lsCmd, false, false)
	flag.SetTicketAccessFlags(lsCmd)
	flag.SetDecryptionFlags(lsCmd)
	flag.SetHiddenFileFlags(lsCmd)
	flag.SetWildcardSearchFlags(lsCmd)

	rootCmd.AddCommand(lsCmd)
}

func processLsCommand(command *cobra.Command, args []string) error {
	ls, err := NewLsCommand(command, args)
	if err != nil {
		return err
	}

	return ls.Process()
}

type LsCommand struct {
	command *cobra.Command

	commonFlagValues         *flag.CommonFlagValues
	outputFormatFlagValues   *flag.OutputFormatFlagValues
	listFlagValues           *flag.ListFlagValues
	ticketAccessFlagValues   *flag.TicketAccessFlagValues
	decryptionFlagValues     *flag.DecryptionFlagValues
	hiddenFileFlagValues     *flag.HiddenFileFlagValues
	wildcardSearchFlagValues *flag.WildcardSearchFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
}

func NewLsCommand(command *cobra.Command, args []string) (*LsCommand, error) {
	ls := &LsCommand{
		command: command,

		commonFlagValues:         flag.GetCommonFlagValues(command),
		outputFormatFlagValues:   flag.GetOutputFormatFlagValues(),
		listFlagValues:           flag.GetListFlagValues(),
		ticketAccessFlagValues:   flag.GetTicketAccessFlagValues(),
		decryptionFlagValues:     flag.GetDecryptionFlagValues(command),
		hiddenFileFlagValues:     flag.GetHiddenFileFlagValues(),
		wildcardSearchFlagValues: flag.GetWildcardSearchFlagValues(),
	}

	// path
	ls.sourcePaths = args[:]

	if len(args) == 0 {
		ls.sourcePaths = []string{"."}
	}

	return ls, nil
}

func (ls *LsCommand) Process() error {
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(ls.command)
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
	ls.account = config.GetSessionConfig().ToIRODSAccount()
	if len(ls.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", ls.ticketAccessFlagValues.Name)
		ls.account.Ticket = ls.ticketAccessFlagValues.Name
	}

	ls.filesystem, err = irods.GetIRODSFSClient(ls.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer ls.filesystem.Release()

	if ls.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(ls.filesystem, ls.commonFlagValues.Timeout)
	}

	// set default key for decryption
	if len(ls.decryptionFlagValues.Key) == 0 {
		ls.decryptionFlagValues.Key = ls.account.Password
	}

	// Expand wildcards
	if ls.wildcardSearchFlagValues.WildcardSearch {
		expanded_results, err := wildcard.ExpandWildcards(ls.filesystem, ls.account, ls.sourcePaths, true, true)
		if err != nil {
			return errors.Wrapf(err, "failed to expand wildcards")
		}
		ls.sourcePaths = expanded_results
	}

	// run
	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())

	for _, sourcePath := range ls.sourcePaths {
		err = ls.listSourcePath(outputFormatter, sourcePath)
		if err != nil {
			return errors.Wrapf(err, "failed to print path %q", sourcePath)
		}
	}

	outputFormatter.Render(ls.outputFormatFlagValues.Format)

	return nil
}

func (ls *LsCommand) requireDecryption(sourcePath string) bool {
	if ls.decryptionFlagValues.NoDecryption {
		return false
	}

	if !ls.decryptionFlagValues.Decryption {
		return false
	}

	mode := encryption.DetectEncryptionMode(sourcePath)
	return mode != encryption.EncryptionModeNone
}

func (ls *LsCommand) listSourcePath(outputFormatter *format.OutputFormatter, sourcePath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := ls.account.ClientZone
	sourcePath = commons_path.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := ls.filesystem.Stat(sourcePath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return errors.Wrapf(err, "failed to find data-object/collection %q", sourcePath)
		}

		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceEntry.IsDir() {
		// dir
		err := ls.listCollection(outputFormatter, sourceEntry)
		if err != nil {
			return errors.Wrapf(err, "failed to list collection %q", sourcePath)
		}
	} else {
		// file
		err := ls.listDataObject(outputFormatter, sourceEntry)
		if err != nil {
			return errors.Wrapf(err, "failed to list data-object %q", sourcePath)
		}
	}

	return nil
}

func (ls *LsCommand) listCollection(outputFormatter *format.OutputFormatter, sourceEntry *irodsclient_fs.Entry) error {
	connection, err := ls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return errors.Wrapf(err, "failed to get connection")
	}
	defer ls.filesystem.ReturnMetadataConnection(connection)

	outputFormatterTable := outputFormatter.NewTable("iRODS Collection")

	// collection
	if ls.listFlagValues.Access {
		// get access
		accesses, err := irodsclient_irodsfs.ListCollectionAccesses(connection, sourceEntry.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to get access for collection %q", sourceEntry.Path)
		}

		inherit, err := irodsclient_irodsfs.GetCollectionAccessInheritance(connection, sourceEntry.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to get access inheritance for collection %q", sourceEntry.Path)
		}

		accessString := ""

		if len(accesses) > 0 {
			accessString = ls.getAccessesString(accesses)
		}

		inheritanceString := "Disabled"
		if inherit != nil {
			if inherit.Inheritance {
				inheritanceString = "Enabled"
			}
		}

		outputFormatterTable.SetHeader([]string{
			"Type",
			"Path",
			"Access",
			"Inheritance",
		})
		outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0})

		outputFormatterTable.AppendRow([]interface{}{
			"collection",
			sourceEntry.Path,
			accessString,
			inheritanceString,
		})
	} else {
		outputFormatterTable.SetHeader([]string{
			"Type",
			"Path",
		})
		outputFormatterTable.SetColumnWidthMax([]int{0, 50})

		outputFormatterTable.AppendRow([]interface{}{
			"collection",
			sourceEntry.Path,
		})
	}

	// sub-collections and data-objects
	colls, err := irodsclient_irodsfs.ListSubCollections(connection, sourceEntry.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to list sub-collections in %q", sourceEntry.Path)
	}

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, sourceEntry.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to list data-objects in %q", sourceEntry.Path)
	}

	// filter out hidden files
	filtered_colls := ls.filterHiddenCollections(colls)
	filtered_objs := ls.filterHiddenDataObjects(objs)

	var accesses []*irodsclient_types.IRODSAccess

	if ls.listFlagValues.Access {
		// get access
		collAccesses, err := irodsclient_irodsfs.ListAccessesForSubCollections(connection, sourceEntry.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to get access for collections in %q", sourceEntry.Path)
		}
		accesses = append(accesses, collAccesses...)

		objectAccesses, err := irodsclient_irodsfs.ListAccessesForDataObjectsInCollection(connection, sourceEntry.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to get access for data-objects in %q", sourceEntry.Path)
		}
		accesses = append(accesses, objectAccesses...)
	}

	ls.printDataObjectsAndCollections(outputFormatter, sourceEntry, filtered_objs, filtered_colls, accesses, false)

	return nil
}

func (ls *LsCommand) listDataObject(outputFormatter *format.OutputFormatter, sourceEntry *irodsclient_fs.Entry) error {
	connection, err := ls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return errors.Wrapf(err, "failed to get connection")
	}
	defer ls.filesystem.ReturnMetadataConnection(connection)

	// data object
	entry, err := irodsclient_irodsfs.GetDataObject(connection, sourceEntry.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to get data-object %q", sourceEntry.Path)
	}

	entries := []*irodsclient_types.IRODSDataObject{entry}

	var accesses []*irodsclient_types.IRODSAccess

	if ls.listFlagValues.Access {
		// get access
		accesses, err = irodsclient_irodsfs.ListDataObjectAccessesWithoutCollection(connection, sourceEntry.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to get access for data-object %q", sourceEntry.Path)
		}
	}

	ls.printDataObjectsAndCollections(outputFormatter, nil, entries, nil, accesses, true)

	return nil
}

func (ls *LsCommand) filterHiddenCollections(entries []*irodsclient_types.IRODSCollection) []*irodsclient_types.IRODSCollection {
	if !ls.hiddenFileFlagValues.Exclude {
		return entries
	}

	filteredEntries := []*irodsclient_types.IRODSCollection{}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, ".") {
			// not hidden
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries
}

func (ls *LsCommand) filterHiddenDataObjects(entries []*irodsclient_types.IRODSDataObject) []*irodsclient_types.IRODSDataObject {
	if !ls.hiddenFileFlagValues.Exclude {
		return entries
	}

	filteredEntries := []*irodsclient_types.IRODSDataObject{}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, ".") {
			// not hidden
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries
}

func (ls *LsCommand) printDataObjectsAndCollections(outputFormatter *format.OutputFormatter, parentEntry *irodsclient_fs.Entry, objectEntries []*irodsclient_types.IRODSDataObject, collectionEntries []*irodsclient_types.IRODSCollection, accesses []*irodsclient_types.IRODSAccess, showFullPath bool) {
	logger := log.WithFields(log.Fields{})

	title := "iRODS Data Object"
	if parentEntry != nil {
		title = fmt.Sprintf("Content of %s", parentEntry.Path)
	}

	outputFormatterTable := outputFormatter.NewTable(title)

	pathTitle := "Name"
	if showFullPath {
		pathTitle = "Path"
	}

	// access is optional
	if ls.listFlagValues.Format == format.ListFormatNormal {
		if ls.listFlagValues.Access {
			outputFormatterTable.SetHeader([]string{
				"Type",
				pathTitle,
				"Access",
				"Description",
			})
			outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 20})
		} else {
			outputFormatterTable.SetHeader([]string{
				"Type",
				pathTitle,
				"Description",
			})
			outputFormatterTable.SetColumnWidthMax([]int{0, 50, 20})
		}

		sort.SliceStable(objectEntries, ls.getDataObjectSortFunction(objectEntries, ls.listFlagValues.SortOrder, ls.listFlagValues.SortReverse))
		sort.SliceStable(collectionEntries, ls.getCollectionSortFunction(collectionEntries, ls.listFlagValues.SortOrder, ls.listFlagValues.SortReverse))

		for _, entry := range objectEntries {
			newName := entry.Name
			if showFullPath {
				newName = entry.Path
			}

			desc := ""
			if ls.requireDecryption(entry.Path) {
				// need to decrypt
				encryptionMode := encryption.DetectEncryptionMode(newName)
				if encryptionMode != encryption.EncryptionModeNone {
					encryptManager := ls.getEncryptionManagerForDecryption(encryptionMode)

					decryptedFilename, err := encryptManager.DecryptFilename(newName)
					if err != nil {
						logger.Debugf("%+v", err)
						desc = "decryption_failed"
					} else {
						desc = fmt.Sprintf("file name: %q", decryptedFilename)
					}
				}
			}

			if ls.listFlagValues.Access {
				accessesForEntry := []*irodsclient_types.IRODSAccess{}
				for _, access := range accesses {
					if access.Path == entry.Path {
						accessesForEntry = append(accessesForEntry, access)
					}
				}

				accessString := ""
				if len(accessesForEntry) > 0 {
					accessString = ls.getAccessesString(accessesForEntry)
				}

				outputFormatterTable.AppendRow([]interface{}{
					"data-object",
					newName,
					accessString,
					desc,
				})
			} else {
				outputFormatterTable.AppendRow([]interface{}{
					"data-object",
					newName,
					desc,
				})
			}
		}

		for _, entry := range collectionEntries {
			newName := entry.Name
			if showFullPath {
				newName = entry.Path
			}

			if ls.listFlagValues.Access {
				accessesForEntry := []*irodsclient_types.IRODSAccess{}
				for _, access := range accesses {
					if access.Path == entry.Path {
						accessesForEntry = append(accessesForEntry, access)
					}
				}

				accessString := ""
				if len(accessesForEntry) > 0 {
					accessString = ls.getAccessesString(accessesForEntry)
				}

				outputFormatterTable.AppendRow([]interface{}{
					"collection",
					newName,
					accessString,
					"",
				})
			} else {
				outputFormatterTable.AppendRow([]interface{}{
					"collection",
					newName,
					"",
				})
			}
		}
	} else {
		switch ls.listFlagValues.Format {
		case format.ListFormatLong:
			if ls.listFlagValues.Access {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Owner",
					"Replica No.",
					"Resource Hierarchy",
					"Size",
					"Modify Time",
					"Status",
					"Access",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0, 20, 0, 0, 0, 0, 20})
			} else {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Owner",
					"Replica No.",
					"Resource Hierarchy",
					"Size",
					"Modify Time",
					"Status",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0, 20, 0, 0, 0, 20})
			}
		case format.ListFormatVeryLong:
			if ls.listFlagValues.Access {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Owner",
					"Replica No.",
					"Resource Hierarchy",
					"Size",
					"Modify Time",
					"Status",
					"Checksum",
					"Replica Path",
					"Access",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0, 20, 0, 0, 0, 0, 50, 0, 20})
			} else {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Owner",
					"Replica No.",
					"Resource Hierarchy",
					"Size",
					"Modify Time",
					"Status",
					"Checksum",
					"Replica Path",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0, 20, 0, 0, 0, 0, 50, 20})
			}
		default:
			if ls.listFlagValues.Access {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Replica No.",
					"Access",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 0, 20})
			} else {
				outputFormatterTable.SetHeader([]string{
					"Type",
					pathTitle,
					"Replica No.",
					"Description",
				})
				outputFormatterTable.SetColumnWidthMax([]int{0, 50, 0, 20})
			}
		}

		// replicas
		var replicas []*FlatReplica
		for _, entry := range objectEntries {
			for _, replica := range entry.Replicas {
				flatReplica := FlatReplica{
					DataObject: entry,
					Replica:    replica,
				}
				replicas = append(replicas, &flatReplica)
			}
		}

		sort.SliceStable(replicas, ls.getFlatReplicaSortFunction(replicas, ls.listFlagValues.SortOrder, ls.listFlagValues.SortReverse))
		sort.SliceStable(collectionEntries, ls.getCollectionSortFunction(collectionEntries, ls.listFlagValues.SortOrder, ls.listFlagValues.SortReverse))

		for _, replica := range replicas {
			newName := replica.DataObject.Name
			if showFullPath {
				newName = replica.DataObject.Path
			}

			desc := ""
			if ls.requireDecryption(replica.DataObject.Path) {
				// need to decrypt
				encryptionMode := encryption.DetectEncryptionMode(newName)
				if encryptionMode != encryption.EncryptionModeNone {
					encryptManager := ls.getEncryptionManagerForDecryption(encryptionMode)

					decryptedFilename, err := encryptManager.DecryptFilename(newName)
					if err != nil {
						logger.Debugf("%+v", err)
						desc = "decryption_failed"
					} else {
						desc = fmt.Sprintf("file name: %q", decryptedFilename)
					}
				}
			}

			size := fmt.Sprintf("%v", replica.DataObject.Size)
			if ls.listFlagValues.HumanReadableSizes {
				size = humanize.Bytes(uint64(replica.DataObject.Size))
			}

			accessString := ""
			if ls.listFlagValues.Access {
				accessesForEntry := []*irodsclient_types.IRODSAccess{}
				for _, access := range accesses {
					if access.Path == replica.DataObject.Path {
						accessesForEntry = append(accessesForEntry, access)
					}
				}

				if len(accessesForEntry) > 0 {
					accessString = ls.getAccessesString(accessesForEntry)
				}
			}

			switch ls.listFlagValues.Format {
			case format.ListFormatLong:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Owner,
						replica.Replica.Number,
						replica.Replica.ResourceHierarchy,
						size,
						types.MakeDateTimeStringHM(replica.Replica.ModifyTime),
						ls.getStatusMark(replica.Replica.Status),
						accessString,
						desc,
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Owner,
						replica.Replica.Number,
						replica.Replica.ResourceHierarchy,
						size,
						types.MakeDateTimeStringHM(replica.Replica.ModifyTime),
						ls.getStatusMark(replica.Replica.Status),
						desc,
					})
				}
			case format.ListFormatVeryLong:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Owner,
						replica.Replica.Number,
						replica.Replica.ResourceHierarchy,
						size,
						types.MakeDateTimeStringHM(replica.Replica.ModifyTime),
						ls.getStatusMark(replica.Replica.Status),
						replica.Replica.Checksum.IRODSChecksumString,
						replica.Replica.Path,
						accessString,
						desc,
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Owner,
						replica.Replica.Number,
						replica.Replica.ResourceHierarchy,
						size,
						types.MakeDateTimeStringHM(replica.Replica.ModifyTime),
						ls.getStatusMark(replica.Replica.Status),
						replica.Replica.Checksum.IRODSChecksumString,
						replica.Replica.Path,
						desc,
					})
				}
			default:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Number,
						accessString,
						desc,
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"data-object",
						newName,
						replica.Replica.Number,
						desc,
					})
				}
			}
		}

		for _, entry := range collectionEntries {
			newName := entry.Name
			if showFullPath {
				newName = entry.Path
			}

			switch ls.listFlagValues.Format {
			case format.ListFormatLong:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						entry.Owner,
						"",
						"",
						"",
						types.MakeDateTimeStringHM(entry.ModifyTime),
						"",
						"",
						"",
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						entry.Owner,
						"",
						"",
						"",
						types.MakeDateTimeStringHM(entry.ModifyTime),
						"",
						"",
					})
				}
			case format.ListFormatVeryLong:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						entry.Owner,
						"",
						"",
						"",
						types.MakeDateTimeStringHM(entry.ModifyTime),
						"",
						"",
						"",
						"",
						"",
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						entry.Owner,
						"",
						"",
						"",
						types.MakeDateTimeStringHM(entry.ModifyTime),
						"",
						"",
						"",
						"",
					})
				}
			default:
				if ls.listFlagValues.Access {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						"",
						"",
					})
				} else {
					outputFormatterTable.AppendRow([]interface{}{
						"collection",
						newName,
						"",
					})
				}
			}
		}
	}
}

func (ls *LsCommand) getFlatReplicaSortFunction(entries []*FlatReplica, sortOrder format.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case format.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].DataObject.Name > entries[j].DataObject.Name
			}
		case format.ListSortOrderExt:
			return func(i int, j int) bool {
				return (path.Ext(entries[i].DataObject.Name) > path.Ext(entries[j].DataObject.Name)) ||
					(path.Ext(entries[i].DataObject.Name) == path.Ext(entries[j].DataObject.Name) &&
						entries[i].DataObject.Name < entries[j].DataObject.Name)
			}
		case format.ListSortOrderTime:
			return func(i int, j int) bool {
				return (entries[i].Replica.ModifyTime.After(entries[j].Replica.ModifyTime)) ||
					(entries[i].Replica.ModifyTime.Equal(entries[j].Replica.ModifyTime) &&
						entries[i].DataObject.Name < entries[j].DataObject.Name)
			}
		case format.ListSortOrderSize:
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
	case format.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].DataObject.Name < entries[j].DataObject.Name
		}
	case format.ListSortOrderExt:
		return func(i int, j int) bool {
			return (path.Ext(entries[i].DataObject.Name) < path.Ext(entries[j].DataObject.Name)) ||
				(path.Ext(entries[i].DataObject.Name) == path.Ext(entries[j].DataObject.Name) &&
					entries[i].DataObject.Name < entries[j].DataObject.Name)
		}
	case format.ListSortOrderTime:
		return func(i int, j int) bool {
			return (entries[i].Replica.ModifyTime.Before(entries[j].Replica.ModifyTime)) ||
				(entries[i].Replica.ModifyTime.Equal(entries[j].Replica.ModifyTime) &&
					entries[i].DataObject.Name < entries[j].DataObject.Name)
		}
	case format.ListSortOrderSize:
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

func (ls *LsCommand) getDataObjectSortFunction(entries []*irodsclient_types.IRODSDataObject, sortOrder format.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case format.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].Name > entries[j].Name
			}
		case format.ListSortOrderExt:
			return func(i int, j int) bool {
				return (path.Ext(entries[i].Name) > path.Ext(entries[j].Name)) ||
					(path.Ext(entries[i].Name) == path.Ext(entries[j].Name) &&
						entries[i].Name < entries[j].Name)
			}
		case format.ListSortOrderTime:
			return func(i int, j int) bool {
				return (ls.getDataObjectModifyTime(entries[i]).After(ls.getDataObjectModifyTime(entries[j]))) ||
					(ls.getDataObjectModifyTime(entries[i]).Equal(ls.getDataObjectModifyTime(entries[j])) &&
						entries[i].Name < entries[j].Name)
			}
		case format.ListSortOrderSize:
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
	case format.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	case format.ListSortOrderExt:
		return func(i int, j int) bool {
			return (path.Ext(entries[i].Name) < path.Ext(entries[j].Name)) ||
				(path.Ext(entries[i].Name) == path.Ext(entries[j].Name) &&
					entries[i].Name < entries[j].Name)
		}
	case format.ListSortOrderTime:
		return func(i int, j int) bool {
			return (ls.getDataObjectModifyTime(entries[i]).Before(ls.getDataObjectModifyTime(entries[j]))) ||
				(ls.getDataObjectModifyTime(entries[i]).Equal(ls.getDataObjectModifyTime(entries[j])) &&
					entries[i].Name < entries[j].Name)
		}
	case format.ListSortOrderSize:
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

func (ls *LsCommand) getDataObjectModifyTime(object *irodsclient_types.IRODSDataObject) time.Time {
	// ModifyTime of data object is considered to be ModifyTime of replica modified most recently
	maxTime := object.Replicas[0].ModifyTime
	for _, t := range object.Replicas[1:] {
		if t.ModifyTime.After(maxTime) {
			maxTime = t.ModifyTime
		}
	}
	return maxTime
}

func (ls *LsCommand) getAccessesString(accesses []*irodsclient_types.IRODSAccess) string {
	// group first then user
	accessStrings := []string{}

	for _, access := range accesses {
		if access.UserType == irodsclient_types.IRODSUserRodsGroup {
			accString := fmt.Sprintf("g:%s#%s:%s", access.UserName, access.UserZone, access.AccessLevel)
			accessStrings = append(accessStrings, accString)
		} else {
			accString := fmt.Sprintf("%s#%s:%s", access.UserName, access.UserZone, access.AccessLevel)
			accessStrings = append(accessStrings, accString)
		}
	}

	return strings.Join(accessStrings, ",\n")
}

func (ls *LsCommand) getEncryptionManagerForDecryption(mode encryption.EncryptionMode) *encryption.EncryptionManager {
	manager := encryption.NewEncryptionManager(mode)

	switch mode {
	case encryption.EncryptionModeWinSCP, encryption.EncryptionModePGP:
		manager.SetKey([]byte(ls.decryptionFlagValues.Key))
	case encryption.EncryptionModeSSH:
		manager.SetPublicPrivateKey(ls.decryptionFlagValues.PrivateKeyPath)
	}

	return manager
}

func (ls *LsCommand) getCollectionSortFunction(entries []*irodsclient_types.IRODSCollection, sortOrder format.ListSortOrder, sortReverse bool) func(i int, j int) bool {
	if sortReverse {
		switch sortOrder {
		case format.ListSortOrderName:
			return func(i int, j int) bool {
				return entries[i].Name > entries[j].Name
			}
		case format.ListSortOrderTime:
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
	case format.ListSortOrderName:
		return func(i int, j int) bool {
			return entries[i].Name < entries[j].Name
		}
	case format.ListSortOrderTime:
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

func (ls *LsCommand) getStatusMark(status string) string {
	switch status {
	case "0":
		return "Stale" // stale
	case "1":
		return "Good" // good
	default:
		return "Unknown"
	}
}

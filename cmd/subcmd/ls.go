package subcmd

import (
	"fmt"
	"path"
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

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

	rootCmd.AddCommand(lsCmd)
}

func processLsCommand(command *cobra.Command, args []string) error {
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
		err = listOne(filesystem, sourcePath, listFlagValues.Format, listFlagValues.HumanReadableSizes)
		if err != nil {
			return xerrors.Errorf("failed to perform ls %s: %w", sourcePath, err)
		}
	}

	return nil
}

func listOne(fs *irodsclient_fs.FileSystem, sourcePath string, format flag.ListFormat, humanReadableSizes bool) error {
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

		printDataObjects(objs, format, humanReadableSizes)
		printCollections(colls)
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

	printDataObject(entry, format, humanReadableSizes)
	return nil
}

func printDataObjects(entries []*irodsclient_types.IRODSDataObject, format flag.ListFormat, humanReadableSizes bool) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		printDataObject(entry, format, humanReadableSizes)
	}
}

func printDataObject(entry *irodsclient_types.IRODSDataObject, format flag.ListFormat, humanReadableSizes bool) {
	size := fmt.Sprintf("%v", entry.Size)
	if humanReadableSizes {
		size = humanize.Bytes(uint64(entry.Size))
	}
	switch format {
	case flag.ListFormatLong:
		for _, replica := range entry.Replicas {
			modTime := commons.MakeDateTimeString(replica.ModifyTime)
			fmt.Printf("  %s\t%d\t%s\t%s\t%s\t%s\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, size, modTime, getStatusMark(replica.Status), entry.Name)
		}
	case flag.ListFormatVeryLong:
		for _, replica := range entry.Replicas {
			modTime := commons.MakeDateTimeString(replica.ModifyTime)
			fmt.Printf("  %s\t%d\t%s\t%s\t%s\t%s\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, size, modTime, getStatusMark(replica.Status), entry.Name)
			fmt.Printf("    %s\t%s\n", replica.CheckSum, replica.Path)
		}
	default:
		fmt.Printf("  %s\n", entry.Name)
	}
}

func printCollections(entries []*irodsclient_types.IRODSCollection) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		fmt.Printf("  C- %s\n", entry.Path)
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

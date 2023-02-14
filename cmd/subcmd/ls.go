package subcmd

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_fs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [collection1] [collection2] ...",
	Short: "List entries in iRODS collections",
	Long:  `This lists data objects and collections in iRODS collections.`,
	RunE:  processLsCommand,
}

func AddLsCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(lsCmd)

	lsCmd.Flags().BoolP("long", "l", false, "List data objects in a long format")
	lsCmd.Flags().BoolP("verylong", "L", false, "List data objects in a very long format")

	rootCmd.AddCommand(lsCmd)
}

func processLsCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processLsCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	longFormat := false
	longFlag := command.Flags().Lookup("long")
	if longFlag != nil {
		longFormat, err = strconv.ParseBool(longFlag.Value.String())
		if err != nil {
			longFormat = false
		}
	}

	veryLongFormat := false
	veryLongFlag := command.Flags().Lookup("verylong")
	if veryLongFlag != nil {
		veryLongFormat, err = strconv.ParseBool(veryLongFlag.Value.String())
		if err != nil {
			veryLongFormat = false
		}
	}

	// Create a file system
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer irodsConn.Disconnect()

	sourcePaths := args[:]

	if len(args) == 0 {
		sourcePaths = []string{"."}
	}

	for _, sourcePath := range sourcePaths {
		err = listOne(irodsConn, sourcePath, longFormat, veryLongFormat)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	return nil
}

func listOne(connection *irodsclient_conn.IRODSConnection, targetPath string, longFormat bool, veryLongFormat bool) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	collection, err := irodsclient_fs.GetCollection(connection, targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return err
		}
	}

	if err == nil {
		colls, err := irodsclient_fs.ListSubCollections(connection, targetPath)
		if err != nil {
			return err
		}

		objs, err := irodsclient_fs.ListDataObjects(connection, collection)
		if err != nil {
			return err
		}

		printDataObjects(objs, veryLongFormat, longFormat)
		printCollections(colls)
		return nil
	}

	// data object
	parentTargetPath := path.Dir(targetPath)

	parentCollection, err := irodsclient_fs.GetCollection(connection, parentTargetPath)
	if err != nil {
		return err
	}

	entry, err := irodsclient_fs.GetDataObject(connection, parentCollection, path.Base(targetPath))
	if err != nil {
		return err
	}

	printDataObject(entry, veryLongFormat, longFormat)
	return nil
}

func printDataObjects(entries []*irodsclient_types.IRODSDataObject, veryLongFormat bool, longFormat bool) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		printDataObject(entry, veryLongFormat, longFormat)
	}
}

func printDataObject(entry *irodsclient_types.IRODSDataObject, veryLongFormat bool, longFormat bool) {
	if veryLongFormat {
		for _, replica := range entry.Replicas {
			modTime := commons.MakeDateTimeString(replica.ModifyTime)
			fmt.Printf("  %s\t%d\t%s\t%d\t%s\t%s\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, entry.Size, modTime, getStatusMark(replica.Status), entry.Name)
			fmt.Printf("    %s\t%s\n", replica.CheckSum, replica.Path)
		}
	} else if longFormat {
		for _, replica := range entry.Replicas {
			modTime := commons.MakeDateTimeString(replica.ModifyTime)
			fmt.Printf("  %s\t%d\t%s\t%d\t%s\t%s\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, entry.Size, modTime, getStatusMark(replica.Status), entry.Name)
		}
	} else {
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

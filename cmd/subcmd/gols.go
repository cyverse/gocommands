package subcmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_fs "github.com/cyverse/go-irodsclient/irods/fs"
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

	// Create a connection
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer irodsConn.Disconnect()

	if len(args) == 0 {
		err = listColletion(irodsConn, ".", longFormat, veryLongFormat)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	} else {
		for _, sourcePath := range args {
			err = listColletion(irodsConn, sourcePath, longFormat, veryLongFormat)
			if err != nil {
				logger.Error(err)
				fmt.Fprintln(os.Stderr, err.Error())
				return nil
			}
		}
	}

	return nil
}

func listColletion(connection *irodsclient_conn.IRODSConnection, collectionPath string, longFormat bool, veryLongFormat bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "listColletion",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	collectionPath = commons.MakeIRODSPath(cwd, home, zone, collectionPath)

	logger.Debugf("listing collection: %s", collectionPath)

	collection, err := irodsclient_fs.GetCollection(connection, collectionPath)
	if err != nil {
		return err
	}

	colls, err := irodsclient_fs.ListSubCollections(connection, collectionPath)
	if err != nil {
		return err
	}

	objs, err := irodsclient_fs.ListDataObjects(connection, collection)
	if err != nil {
		return err
	}

	// sort by name
	sort.SliceStable(objs, func(i int, j int) bool {
		return objs[i].Name < objs[j].Name
	})

	sort.SliceStable(colls, func(i int, j int) bool {
		return colls[i].Name < colls[j].Name
	})

	// print data objects first
	for _, entry := range objs {
		if veryLongFormat {
			for _, replica := range entry.Replicas {
				modTime := commons.MakeDateTimeString(replica.ModifyTime)
				fmt.Printf("  %s\t%d\t%s\t%d\t%s\t&\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, entry.Size, modTime, entry.Name)
				fmt.Printf("    %s\t%s\n", replica.CheckSum, replica.Path)
			}
		} else if longFormat {
			for _, replica := range entry.Replicas {
				modTime := commons.MakeDateTimeString(replica.ModifyTime)
				fmt.Printf("  %s\t%d\t%s\t%d\t%s\t&\t%s\n", replica.Owner, replica.Number, replica.ResourceHierarchy, entry.Size, modTime, entry.Name)
			}
		} else {
			fmt.Printf("  %s\n", entry.Name)
		}
	}

	// print collections next
	for _, entry := range colls {
		fmt.Printf("  C- %s\n", entry.Path)
	}
	return nil
}

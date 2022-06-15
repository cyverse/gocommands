package subcmd

import (
	"fmt"
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
	Short: "List current iRODS collection",
	Long:  `This lists data objects and collections in current iRODS collection.`,
	RunE:  processLsCommand,
}

func AddLsCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(lsCmd)

	lsCmd.Flags().BoolP("long", "l", false, "List data objects in long formnat")

	rootCmd.AddCommand(lsCmd)
}

func processLsCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processLsCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		logger.Error(err)
	}

	if !cont {
		return err
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		return err
	}

	longFormat := false
	longFlag := command.Flags().Lookup("long")
	if longFlag != nil {
		longFormat, err = strconv.ParseBool(longFlag.Value.String())
		if err != nil {
			longFormat = false
		}
	}

	// Create a connection
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		return err
	}

	defer irodsConn.Disconnect()

	if len(args) == 0 {
		err = listColletion(irodsConn, ".", longFormat)
		if err != nil {
			logger.Error(err)
			return err
		}
	} else {
		for _, sourcePath := range args {
			err = listColletion(irodsConn, sourcePath, longFormat)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	}

	return nil
}

func listColletion(connection *irodsclient_conn.IRODSConnection, collectionPath string, longFormat bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "listColletion",
	})

	cwd := commons.GetCWD()
	collectionPath = commons.MakeIRODSPath(cwd, collectionPath)

	logger.Debugf("listing collection: %s\n", collectionPath)

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
		if longFormat {
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

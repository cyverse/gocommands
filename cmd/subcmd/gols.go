package subcmd

import (
	"fmt"
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) == 0 {
		err = listColletion(filesystem, ".")
		if err != nil {
			logger.Error(err)
			return err
		}
	} else {
		for _, sourcePath := range args {
			err = listColletion(filesystem, sourcePath)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	}

	return nil
}

func listColletion(filesystem *irodsclient_fs.FileSystem, collectionPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "listColletion",
	})

	cwd := commons.GetCWD()
	collectionPath = commons.MakeIRODSPath(cwd, collectionPath)

	logger.Debugf("listing collection: %s\n", collectionPath)

	entries, err := filesystem.List(collectionPath)
	if err != nil {
		return err
	}

	fmt.Printf("%s:\n", collectionPath)
	objs := []*irodsclient_fs.Entry{}
	colls := []*irodsclient_fs.Entry{}

	for _, entry := range entries {
		if entry.Type == irodsclient_fs.FileEntry {
			objs = append(objs, entry)
		} else {
			// dir
			colls = append(colls, entry)
		}
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
		fmt.Printf("  %s\n", entry.Name)
	}

	// print collections next
	for _, entry := range colls {
		fmt.Printf("  C- %s\n", entry.Path)
	}
	return nil
}

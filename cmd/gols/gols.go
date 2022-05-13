package main

import (
	"fmt"
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gols",
	Short: "List current iRODS collection",
	Long:  `This lists data objects and collections in current iRODS collection.`,
	RunE:  processCommand,
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
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

	if len(args) == 0 {
		err = listCurrentCollection()
		if err != nil {
			logger.Error(err)
			return err
		}
	} else {
		for _, coll := range args {
			err = listColletion(coll)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	}

	return nil
}

func main() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	commons.SetCommonFlags(rootCmd)

	err := Execute()
	if err != nil {
		logger.Fatal(err)
		os.Exit(1)
	}
}

func listCurrentCollection() error {
	envMgr := commons.GetEnvironmentManager()
	env := envMgr.Environment
	currentWorkingDir := env.CurrentWorkingDir
	if len(currentWorkingDir) == 0 {
		// set new
		currentWorkingDir = fmt.Sprintf("/%s/home/%s", env.Zone, env.Username)
	}

	return listColletion(currentWorkingDir)
}

func listColletion(collectionPath string) error {
	collectionPath = filepath.Clean(collectionPath)

	// Create a file system
	account := commons.GetAccount()

	filesystem, err := irodsclient_fs.NewFileSystemWithDefault(account, "gocommands-ls")
	if err != nil {
		return err
	}

	defer filesystem.Release()

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

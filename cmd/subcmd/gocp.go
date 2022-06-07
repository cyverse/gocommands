package subcmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:   "cp [data-object1] [data-object2] [collection1] ... [target collection]",
	Short: "Copy iRODS data-objects or collections to target collection",
	Long:  `This copies iRODS data-objects or collections to the given target collection.`,
	RunE:  processCpCommand,
}

func AddCpCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(cpCmd)

	cpCmd.Flags().BoolP("recurse", "r", false, "Copy recursively")

	rootCmd.AddCommand(cpCmd)
}

func processCpCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCpCommand",
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

	recurse := false
	recurseFlag := command.Flags().Lookup("recurse")
	if recurseFlag != nil {
		recurse, err = strconv.ParseBool(recurseFlag.Value.String())
		if err != nil {
			recurse = false
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) == 2 {
		// copy to another
		err = copyOne(filesystem, args[0], args[1], recurse)
		if err != nil {
			logger.Error(err)
			return err
		}
	} else if len(args) >= 3 {
		// copy
		destPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = copyOne(filesystem, sourcePath, destPath, recurse)
			if err != nil {
				logger.Error(err)
				return err
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}
	return nil
}

func copyOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, recurse bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "copyOne",
	})

	cwd := commons.GetCWD()
	sourcePath = commons.MakeIRODSPath(cwd, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("copying a data object %s to %s\n", sourcePath, targetPath)
		err = filesystem.CopyFile(sourcePath, targetPath)
		if err != nil {
			return err
		}
	} else {
		// dir
		if !recurse {
			return fmt.Errorf("cannot copy a collection, recurse is set")
		}

		logger.Debugf("copying a collection %s to %s\n", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return err
		}

		if !filesystem.ExistsDir(targetPath) {
			// make target dir
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return err
			}

			for _, entryInDir := range entries {
				err = copyOne(filesystem, entryInDir.Path, targetPath, recurse)
				if err != nil {
					return err
				}
			}
		} else {
			// make a sub dir
			targetDir := filepath.Join(targetPath, sourceEntry.Name)
			err = filesystem.MakeDir(targetDir, true)
			if err != nil {
				return err
			}

			for _, entryInDir := range entries {
				err = copyOne(filesystem, entryInDir.Path, targetDir, recurse)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

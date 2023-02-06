package subcmd

import (
	"fmt"
	"os"
	"path"
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
	cpCmd.Flags().BoolP("force", "f", false, "Copy forcefully")
	cpCmd.Flags().Bool("progress", false, "Display progress bar")

	rootCmd.AddCommand(cpCmd)
}

func processCpCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCpCommand",
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

	recurse := false
	recurseFlag := command.Flags().Lookup("recurse")
	if recurseFlag != nil {
		recurse, err = strconv.ParseBool(recurseFlag.Value.String())
		if err != nil {
			recurse = false
		}
	}

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	progress := false
	progressFlag := command.Flags().Lookup("progress")
	if progressFlag != nil {
		progress, err = strconv.ParseBool(progressFlag.Value.String())
		if err != nil {
			progress = false
		}
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	parallelTransferManager := commons.NewParallelTransferManager(commons.MaxThreadNum)

	if len(args) == 2 {
		// copy to another
		err = copyOne(parallelTransferManager, filesystem, args[0], args[1], recurse, force)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	} else if len(args) >= 3 {
		// copy
		destPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			err = copyOne(parallelTransferManager, filesystem, sourcePath, destPath, recurse, force)
			if err != nil {
				logger.Error(err)
				fmt.Fprintln(os.Stderr, err.Error())
				return nil
			}
		}
	} else {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	err = parallelTransferManager.Go(progress)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func copyOne(transferManager *commons.ParallelTransferManager, filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, recurse bool, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "copyOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		if filesystem.ExistsFile(targetFilePath) {
			// already exists!
			if force {
				// delete first
				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s", targetFilePath)
					err := filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip copying a file %s. The file already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		logger.Debugf("scheduled a data object copy %s to %s", sourcePath, targetFilePath)
		transferManager.ScheduleCopy(filesystem, sourcePath, targetFilePath)
	} else {
		// dir
		if !recurse {
			return fmt.Errorf("cannot copy a collection, turn on 'recurse' option")
		}

		logger.Debugf("copying a collection %s to %s", sourcePath, targetPath)

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
				err = copyOne(transferManager, filesystem, entryInDir.Path, targetPath, recurse, force)
				if err != nil {
					return err
				}
			}
		} else {
			// make a sub dir
			targetDir := path.Join(targetPath, sourceEntry.Name)
			if !filesystem.ExistsDir(targetDir) {
				err = filesystem.MakeDir(targetDir, true)
				if err != nil {
					return err
				}
			}

			for _, entryInDir := range entries {
				err = copyOne(transferManager, filesystem, entryInDir.Path, targetDir, recurse, force)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

package subcmd

import (
	"fmt"
	"os"
	"path"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
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
	cpCmd.Flags().Bool("progress", false, "Display progress bars")
	getCmd.Flags().Bool("diff", false, "Copy files having different content")
	getCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")

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

	diff := false
	diffFlag := command.Flags().Lookup("diff")
	if diffFlag != nil {
		diff, err = strconv.ParseBool(diffFlag.Value.String())
		if err != nil {
			diff = false
		}
	}

	noHash := false
	noHashFlag := command.Flags().Lookup("no_hash")
	if noHashFlag != nil {
		noHash, err = strconv.ParseBool(noHashFlag.Value.String())
		if err != nil {
			noHash = false
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

	if len(args) <= 1 {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	targetPath := args[len(args)-1]
	sourcePaths := args[:len(args)-1]

	parallelJobManager := commons.NewParallelJobManager(filesystem, commons.MaxThreadNumDefault, progress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		err = copyOne(parallelJobManager, sourcePath, targetPath, recurse, force, diff, noHash)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func copyOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, recurse bool, force bool, diff bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "copyOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		exist := filesystem.ExistsFile(targetFilePath)

		copyTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			job.Progress(0, 1, false)

			logger.Debugf("copying a data object %s to %s", sourcePath, targetFilePath)
			err = fs.CopyFileToFile(sourcePath, targetFilePath)
			if err != nil {
				job.Progress(-1, 1, true)
				return err
			}

			logger.Debugf("copied a data object %s to %s", sourcePath, targetFilePath)
			job.Progress(1, 1, false)
			return nil
		}

		if exist {
			targetEntry, err := filesystem.Stat(targetFilePath)
			if err != nil {
				return err
			}

			if diff {
				if noHash {
					if targetEntry.Size == sourceEntry.Size {
						fmt.Printf("skip copying a file %s. The file already exists!\n", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size == sourceEntry.Size {
						// compare hash
						if len(sourceEntry.CheckSum) > 0 && sourceEntry.CheckSum == targetEntry.CheckSum {
							fmt.Printf("skip copying a file %s. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}

				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err = filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			} else if force {
				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err = filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s", targetFilePath)
					err = filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip copying a file %s. The file already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		parallelJobManager.Schedule(sourcePath, copyTask, 1, progress.UnitsDefault)
		logger.Debugf("scheduled a data object copy %s to %s", sourcePath, targetFilePath)
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
				err = copyOne(parallelJobManager, entryInDir.Path, targetPath, recurse, force, diff, noHash)
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
				err = copyOne(parallelJobManager, entryInDir.Path, targetDir, recurse, force, diff, noHash)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

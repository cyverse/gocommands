package subcmd

import (
	"fmt"
	"path"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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
	cpCmd.Flags().Bool("diff", false, "Copy files having different content")
	cpCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")
	cpCmd.Flags().Int("retry", 1, "Retry if fails (default is 1)")
	cpCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds (default is 60)")

	rootCmd.AddCommand(cpCmd)
}

func processCpCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
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

	retryChild := false
	retryChildFlag := command.Flags().Lookup("retry_child")
	if retryChildFlag != nil {
		retryChildValue, err := strconv.ParseBool(retryChildFlag.Value.String())
		if err != nil {
			retryChildValue = false
		}

		retryChild = retryChildValue
	}

	retry := int64(1)
	retryFlag := command.Flags().Lookup("retry")
	if retryFlag != nil {
		retry, err = strconv.ParseInt(retryFlag.Value.String(), 10, 32)
		if err != nil {
			retry = 1
		}
	}

	retryInterval := int64(60)
	retryIntervalFlag := command.Flags().Lookup("retry_interval")
	if retryIntervalFlag != nil {
		retryInterval, err = strconv.ParseInt(retryIntervalFlag.Value.String(), 10, 32)
		if err != nil {
			retryInterval = 60
		}
	}

	if retry > 1 && !retryChild {
		err = commons.RunWithRetry(int(retry), int(retryInterval))
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retry, err)
		}
		return nil
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(args) <= 1 {
		return xerrors.Errorf("not enough input arguments")
	}

	targetPath := args[len(args)-1]
	sourcePaths := args[:len(args)-1]

	parallelJobManager := commons.NewParallelJobManager(filesystem, commons.MaxParallelJobThreadNumDefault, progress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		err = copyOne(parallelJobManager, sourcePath, targetPath, recurse, force, diff, noHash)
		if err != nil {
			return xerrors.Errorf("failed to perform cp %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel job: %w", err)
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

	sourceEntry, err := commons.StatIRODSPath(filesystem, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		exist := commons.ExistsIRODSFile(filesystem, targetFilePath)

		copyTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			job.Progress(0, 1, false)

			logger.Debugf("copying a data object %s to %s", sourcePath, targetFilePath)
			err = fs.CopyFileToFile(sourcePath, targetFilePath)
			if err != nil {
				job.Progress(-1, 1, true)
				return xerrors.Errorf("failed to copy %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("copied a data object %s to %s", sourcePath, targetFilePath)
			job.Progress(1, 1, false)
			return nil
		}

		if exist {
			targetEntry, err := commons.StatIRODSPath(filesystem, targetFilePath)
			if err != nil {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
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
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else if force {
				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err = filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s", targetFilePath)
					err = filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
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
			return xerrors.Errorf("cannot copy a collection, turn on 'recurse' option")
		}

		logger.Debugf("copying a collection %s to %s", sourcePath, targetPath)

		entries, err := commons.ListIRODSDir(filesystem, sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		if !commons.ExistsIRODSDir(filesystem, targetPath) {
			// make target dir
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make dir %s: %w", targetPath, err)
			}

			for _, entryInDir := range entries {
				err = copyOne(parallelJobManager, entryInDir.Path, targetPath, recurse, force, diff, noHash)
				if err != nil {
					return xerrors.Errorf("failed to perform copy %s to %s: %w", entryInDir.Path, targetPath, err)
				}
			}
		} else {
			// make a sub dir
			targetDir := path.Join(targetPath, sourceEntry.Name)
			if !commons.ExistsIRODSDir(filesystem, targetDir) {
				err = filesystem.MakeDir(targetDir, true)
				if err != nil {
					return xerrors.Errorf("failed to make dir %s: %w", targetPath, err)
				}
			}

			for _, entryInDir := range entries {
				err = copyOne(parallelJobManager, entryInDir.Path, targetDir, recurse, force, diff, noHash)
				if err != nil {
					return xerrors.Errorf("failed to perform copy %s to %s: %w", entryInDir.Path, targetDir, err)
				}
			}
		}
	}
	return nil
}

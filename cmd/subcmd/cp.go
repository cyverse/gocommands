package subcmd

import (
	"fmt"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var cpCmd = &cobra.Command{
	Use:     "cp [data-object1] [data-object2] [collection1] ... [target collection]",
	Aliases: []string{"icp", "copy"},
	Short:   "Copy iRODS data-objects or collections to target collection",
	Long:    `This copies iRODS data-objects or collections to the given target collection.`,
	RunE:    processCpCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddCpCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(cpCmd)

	flag.SetForceFlags(cpCmd, false)
	flag.SetRecursiveFlags(cpCmd)
	flag.SetProgressFlags(cpCmd)
	flag.SetRetryFlags(cpCmd)
	flag.SetDifferentialTransferFlags(cpCmd, true)
	flag.SetNoRootFlags(cpCmd)
	flag.SetSyncFlags(cpCmd)

	rootCmd.AddCommand(cpCmd)
}

func processCpCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCpCommand",
	})

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

	recursiveFlagValues := flag.GetRecursiveFlagValues()
	forceFlagValues := flag.GetForceFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()
	differentialTransferFlagValues := flag.GetDifferentialTransferFlagValues()
	noRootFlagValues := flag.GetNoRootFlagValues()
	syncFlagValues := flag.GetSyncFlagValues()

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
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

	targetPath := args[len(args)-1]
	sourcePaths := args[:len(args)-1]

	if noRootFlagValues.NoRoot && len(sourcePaths) > 1 {
		return xerrors.Errorf("failed to copy multiple source collections without creating root directory")
	}

	parallelJobManager := commons.NewParallelJobManager(filesystem, commons.TransferTreadNumDefault, progressFlagValues.ShowProgress)
	parallelJobManager.Start()

	inputPathMap := map[string]bool{}

	for _, sourcePath := range sourcePaths {
		newTargetDirPath, err := makeCopyTargetDirPath(filesystem, sourcePath, targetPath, noRootFlagValues.NoRoot)
		if err != nil {
			return xerrors.Errorf("failed to make new target path for copy %s to %s: %w", sourcePath, targetPath, err)
		}

		err = copyOne(parallelJobManager, inputPathMap, sourcePath, newTargetDirPath, recursiveFlagValues.Recursive, forceFlagValues.Force, differentialTransferFlagValues.DifferentialTransfer, differentialTransferFlagValues.NoHash)
		if err != nil {
			return xerrors.Errorf("failed to perform copy %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel job: %w", err)
	}

	// delete extra
	if syncFlagValues.Delete {
		logger.Infof("deleting extra files and dirs under %s", targetPath)

		err = copyDeleteExtra(filesystem, inputPathMap, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func copyOne(parallelJobManager *commons.ParallelJobManager, inputPathMap map[string]bool, sourcePath string, targetPath string, recurse bool, force bool, diff bool, noHash bool) error {
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
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	inputPathMap[targetPath] = true

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		inputPathMap[targetFilePath] = true

		fileExist := false
		targetEntry, err := filesystem.StatFile(targetFilePath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			fileExist = true
		}

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

		if fileExist {
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

				// TODO: Check if we can overwrite without remove
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

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		for _, entry := range entries {
			targetDirPath := targetPath
			if entry.Type == irodsclient_fs.DirectoryEntry {
				// dir
				targetDirPath = commons.MakeTargetIRODSFilePath(filesystem, entry.Path, targetPath)
				err = filesystem.MakeDir(targetDirPath, true)
				if err != nil {
					return xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
				}
			}

			inputPathMap[targetDirPath] = true

			err = copyOne(parallelJobManager, inputPathMap, entry.Path, targetDirPath, recurse, force, diff, noHash)
			if err != nil {
				return xerrors.Errorf("failed to perform copy %s to %s: %w", entry.Path, targetPath, err)
			}
		}
	}
	return nil
}

func makeCopyTargetDirPath(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, noRoot bool) (string, error) {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return "", xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := filesystem.Stat(targetDirPath)
		if err != nil {
			return "", xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		return targetDirPath, nil
	} else {
		// dir
		targetDirPath := targetPath

		if filesystem.ExistsDir(targetDirPath) {
			// already exist
			if !noRoot {
				targetDirPath = commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetDirPath)
				err = filesystem.MakeDir(targetDirPath, true)
				if err != nil {
					return "", xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
				}
			}
		} else {
			err = filesystem.MakeDir(targetDirPath, true)
			if err != nil {
				return "", xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
			}
		}

		return targetDirPath, nil
	}
}

func copyDeleteExtra(filesystem *irodsclient_fs.FileSystem, inputPathMap map[string]bool, targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	return copyDeleteExtraInternal(filesystem, inputPathMap, targetPath)
}

func copyDeleteExtraInternal(filesystem *irodsclient_fs.FileSystem, inputPathMap map[string]bool, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "copyDeleteExtraInternal",
	})

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra data object %s", targetPath)
			removeErr := filesystem.RemoveFile(targetPath, true)
			if removeErr != nil {
				return removeErr
			}
		}
	} else {
		// dir
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra dir
			logger.Debugf("removing an extra collection %s", targetPath)
			removeErr := filesystem.RemoveDir(targetPath, true, true)
			if removeErr != nil {
				return removeErr
			}
		} else {
			// non extra dir
			entries, err := filesystem.List(targetPath)
			if err != nil {
				return xerrors.Errorf("failed to list dir %s: %w", targetPath, err)
			}

			for idx := range entries {
				newTargetPath := entries[idx].Path

				err = copyDeleteExtraInternal(filesystem, inputPathMap, newTargetPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

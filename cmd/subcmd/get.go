package subcmd

import (
	"fmt"
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var getCmd = &cobra.Command{
	Use:     "get [data-object1] [data-object2] [collection1] ... [local dir]",
	Aliases: []string{"iget", "download"},
	Short:   "Download iRODS data-objects or collections",
	Long:    `This downloads iRODS data-objects or collections to the given local path.`,
	RunE:    processGetCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(getCmd)

	flag.SetForceFlags(getCmd, false)
	flag.SetParallelTransferFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetDifferentialTransferFlags(getCmd, true)

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
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

	forceFlagValues := flag.GetForceFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()
	differentialTransferFlagValues := flag.GetDifferentialTransferFlagValues()

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := "./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, progressFlagValues.ShowProgress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		err = getOne(parallelJobManager, sourcePath, targetPath, forceFlagValues.Force, differentialTransferFlagValues.DifferentialTransfer, differentialTransferFlagValues.NoHash)
		if err != nil {
			return xerrors.Errorf("failed to perform get %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	return nil
}

func getOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, force bool, diff bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceEntry, err := commons.StatIRODSPath(filesystem, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)

		exist := false
		targetEntry, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			exist = true
		}

		getTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackGet := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceEntry.Size, false)

			logger.Debugf("downloading a data object %s to %s", sourcePath, targetFilePath)
			err := fs.DownloadFileParallel(sourcePath, "", targetFilePath, 0, callbackGet)
			if err != nil {
				job.Progress(-1, sourceEntry.Size, true)
				return xerrors.Errorf("failed to download %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("downloaded a data object %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceEntry.Size, sourceEntry.Size, false)
			return nil
		}

		if exist {
			if diff {
				if noHash {
					if targetEntry.Size() == sourceEntry.Size {
						fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size() == sourceEntry.Size {
						if len(sourceEntry.CheckSum) > 0 {
							// compare hash
							hash, err := commons.HashLocalFile(targetFilePath, sourceEntry.CheckSumAlgorithm)
							if err != nil {
								return xerrors.Errorf("failed to get hash of %s: %w", targetFilePath, err)
							}

							if sourceEntry.CheckSum == hash {
								fmt.Printf("skip downloading a data object %s. The file with the same hash already exists!\n", targetFilePath)
								return nil
							}
						}
					}
				}

				logger.Debugf("deleting an existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else if force {
				logger.Debugf("deleting an existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing file %s", targetFilePath)
					err := os.Remove(targetFilePath)
					if err != nil {
						return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
					}
				} else {
					fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
		parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a data object download %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := commons.ListIRODSDir(filesystem, sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		// make target dir
		targetDir := filepath.Join(targetPath, sourceEntry.Name)
		err = os.MkdirAll(targetDir, 0766)
		if err != nil {
			return xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
		}

		for idx := range entries {
			path := entries[idx].Path

			err = getOne(parallelJobManager, path, targetDir, force, diff, noHash)
			if err != nil {
				return xerrors.Errorf("failed to perform get %s to %s: %w", path, targetDir, err)
			}
		}
	}
	return nil
}

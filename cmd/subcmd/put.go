package subcmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var putCmd = &cobra.Command{
	Use:     "put [local file1] [local file2] [local dir1] ... [collection]",
	Aliases: []string{"iput", "upload"},
	Short:   "Upload files or directories",
	Long:    `This uploads files or directories to the given iRODS collection.`,
	RunE:    processPutCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(putCmd)

	flag.SetForceFlags(putCmd, false)
	flag.SetTicketAccessFlags(putCmd)
	flag.SetParallelTransferFlags(putCmd, true)
	flag.SetProgressFlags(putCmd)
	flag.SetRetryFlags(putCmd)
	flag.SetDifferentialTransferFlags(putCmd, true)
	flag.SetNoRootFlags(putCmd)
	flag.SetSyncFlags(putCmd)

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processPutCommand",
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

	forceFlagValues := flag.GetForceFlagValues()
	ticketAccessFlagValues := flag.GetTicketAccessFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()
	differentialTransferFlagValues := flag.GetDifferentialTransferFlagValues()
	noRootFlagValues := flag.GetNoRootFlagValues()
	syncFlagValues := flag.GetSyncFlagValues()

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	appConfig := commons.GetConfig()
	syncAccount := false
	if len(ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %s", ticketAccessFlagValues.Name)
		appConfig.Ticket = ticketAccessFlagValues.Name
		syncAccount = true
	}

	if syncAccount {
		err := commons.SyncAccount()
		if err != nil {
			return err
		}
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

	if noRootFlagValues.NoRoot && len(sourcePaths) > 1 {
		return xerrors.Errorf("failed to put multiple source dirs without creating root directory")
	}

	parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, progressFlagValues.ShowProgress)
	parallelJobManager.Start()

	inputPathMap := map[string]bool{}

	for _, sourcePath := range sourcePaths {
		err = putOne(parallelJobManager, inputPathMap, sourcePath, targetPath, forceFlagValues.Force, parallelTransferFlagValues.SingleTread, differentialTransferFlagValues.DifferentialTransfer, differentialTransferFlagValues.NoHash, noRootFlagValues.NoRoot)
		if err != nil {
			return xerrors.Errorf("failed to perform put %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// delete extra
	if syncFlagValues.Delete {
		logger.Infof("deleting extra files and dirs under %s", targetPath)

		err = putDeleteExtra(filesystem, inputPathMap, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func putOne(parallelJobManager *commons.ParallelJobManager, inputPathMap map[string]bool, sourcePath string, targetPath string, force bool, singleThreaded bool, diff bool, noHash bool, noRoot bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := filesystem.Stat(targetDirPath)
		if err != nil {
			return xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		inputPathMap[targetFilePath] = true

		fileExist := filesystem.ExistsFile(targetFilePath)

		putTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackPut := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceStat.Size(), false)

			logger.Debugf("uploading a file %s to %s", sourcePath, targetFilePath)
			if singleThreaded {
				err = fs.UploadFile(sourcePath, targetFilePath, "", true, callbackPut)
			} else {
				err = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, true, callbackPut)
			}

			if err != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to upload %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("uploaded a file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceStat.Size(), sourceStat.Size(), false)
			return nil
		}

		if fileExist {
			targetEntry, err := filesystem.Stat(targetFilePath)
			if err != nil {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}

			if diff {
				if noHash {
					if targetEntry.Size == sourceStat.Size() {
						fmt.Printf("skip uploading a file %s. The file already exists!\n", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size == sourceStat.Size() {
						if len(targetEntry.CheckSum) > 0 {
							// compare hash
							hash, err := commons.HashLocalFile(sourcePath, targetEntry.CheckSumAlgorithm)
							if err != nil {
								return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
							}

							if hash == targetEntry.CheckSum {
								fmt.Printf("skip uploading a file %s. The file with the same hash already exists!\n", targetFilePath)
								return nil
							}
						}
					}
				}
			} else {
				if !force {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
					if !overwrite {
						fmt.Printf("skip uploading a file %s. The data object already exists!\n", targetFilePath)
						return nil
					}
				}
			}
		}

		threadsRequired := computeThreadsRequiredForPut(filesystem, singleThreaded, sourceStat.Size())
		parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a local file upload %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		_, err := filesystem.Stat(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

		logger.Debugf("uploading a local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		targetDir := targetPath
		inputPathMap[targetDir] = true

		if !noRoot {
			// make target dir
			targetDir := path.Join(targetPath, filepath.Base(sourcePath))
			err = filesystem.MakeDir(targetDir, true)
			if err != nil {
				return xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
			}
		}

		for _, entryInDir := range entries {
			newSourcePath := filepath.Join(sourcePath, entryInDir.Name())
			err = putOne(parallelJobManager, inputPathMap, newSourcePath, targetDir, force, singleThreaded, diff, noHash, false)
			if err != nil {
				return xerrors.Errorf("failed to perform put %s to %s: %w", newSourcePath, targetDir, err)
			}
		}
	}
	return nil
}

func computeThreadsRequiredForPut(fs *irodsclient_fs.FileSystem, singleThreaded bool, size int64) int {
	if singleThreaded {
		return 1
	}

	if fs.SupportParallelUpload() {
		return irodsclient_util.GetNumTasksForParallelTransfer(size)
	}

	return 1
}

func putDeleteExtra(filesystem *irodsclient_fs.FileSystem, inputPathMap map[string]bool, targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	return putDeleteExtraInternal(filesystem, inputPathMap, targetPath)
}

func putDeleteExtraInternal(filesystem *irodsclient_fs.FileSystem, inputPathMap map[string]bool, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putDeleteExtraInternal",
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

				err = putDeleteExtraInternal(filesystem, inputPathMap, newTargetPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

package subcmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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
	flag.SetCommonFlags(getCmd, false)

	flag.SetForceFlags(getCmd, false)
	flag.SetTicketAccessFlags(getCmd)
	flag.SetParallelTransferFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetDifferentialTransferFlags(getCmd, true)
	flag.SetNoRootFlags(getCmd)
	flag.SetSyncFlags(getCmd)
	flag.SetDecryptionFlags(getCmd)
	flag.SetPostTransferFlagValues(getCmd)

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "processGetCommand",
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
	decryptionFlagValues := flag.GetDecryptionFlagValues()
	postTransferFlagValues := flag.GetPostTransferFlagValues()

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

	// set default key for decryption
	if len(decryptionFlagValues.Key) == 0 {
		decryptionFlagValues.Key = account.Password
	}

	targetPath := "./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	if noRootFlagValues.NoRoot && len(sourcePaths) > 1 {
		return xerrors.Errorf("failed to get multiple source collections without creating root directory")
	}

	parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, progressFlagValues.ShowProgress)
	parallelJobManager.Start()

	inputPathMap := map[string]bool{}

	for _, sourcePath := range sourcePaths {
		newTargetDirPath, err := makeGetTargetDirPath(filesystem, sourcePath, targetPath, noRootFlagValues.NoRoot)
		if err != nil {
			return xerrors.Errorf("failed to make new target path for get %s to %s: %w", sourcePath, targetPath, err)
		}

		err = getOne(parallelJobManager, inputPathMap, sourcePath, newTargetDirPath, forceFlagValues, parallelTransferFlagValues, differentialTransferFlagValues, decryptionFlagValues, postTransferFlagValues)
		if err != nil {
			return xerrors.Errorf("failed to perform get %s to %s: %w", sourcePath, targetPath, err)
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

		err = getDeleteExtra(inputPathMap, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func getOne(parallelJobManager *commons.ParallelJobManager, inputPathMap map[string]bool, sourcePath string, targetPath string, forceFlagValues *flag.ForceFlagValues, parallelTransferFlagValues *flag.ParallelTransferFlagValues, differentialTransferFlagValues *flag.DifferentialTransferFlagValues, decryptionFlagValues *flag.DecryptionFlagValues, postTransferFlagValues *flag.PostTransferFlagValues) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "getOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	originalSourcePath := sourcePath

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		decryptedTargetFilePath := targetFilePath

		// decrypt first if necessary
		encryptionMode, encryptFilename := commons.DetectEncryptionMode(sourceEntry.Name)
		if encryptionMode == commons.EncryptionModeUnknown {
			// filename doesn't have .pgp.enc
			encryptionMode = commons.EncryptionModePGP
			encryptFilename = false
		}

		encryptManager := commons.NewEncryptionManager(encryptionMode, encryptFilename, []byte(decryptionFlagValues.Key))

		if decryptionFlagValues.Decryption {
			targetFilePath = filepath.Join(decryptionFlagValues.TempPath, sourceEntry.Name)

			newFilename, err := encryptManager.DecryptFilename(sourceEntry.Name)
			if err != nil {
				return xerrors.Errorf("failed to decrypt %s: %w", targetFilePath, err)
			}

			decryptedTargetFilePath = commons.MakeTargetLocalFilePath(newFilename, targetPath)

			logger.Debugf("downloading a decrypted file to %s", decryptedTargetFilePath)
		}

		commons.MarkPathMap(inputPathMap, decryptedTargetFilePath)

		fileExist := false
		targetEntry, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			fileExist = true
		}

		getTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackGet := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceEntry.Size, false)

			logger.Debugf("downloading a data object %s to %s", sourcePath, targetFilePath)

			var downloadErr error

			// determine how to download

			if parallelTransferFlagValues.SingleTread || parallelTransferFlagValues.ThreadNumber == 1 {
				downloadErr = fs.DownloadFileResumable(sourcePath, "", targetFilePath, callbackGet)
			} else if parallelTransferFlagValues.RedirectToResource {
				downloadErr = fs.DownloadFileRedirectToResource(sourcePath, "", targetFilePath, callbackGet)
			} else if parallelTransferFlagValues.Icat {
				downloadErr = fs.DownloadFileParallelResumable(sourcePath, "", targetFilePath, 0, callbackGet)
			} else {
				// auto
				if sourceEntry.Size >= commons.RedirectToResourceMinSize {
					// redirect-to-resource
					downloadErr = fs.DownloadFileRedirectToResource(sourcePath, "", targetFilePath, callbackGet)
				} else {
					downloadErr = fs.DownloadFileParallelResumable(sourcePath, "", targetFilePath, 0, callbackGet)
				}
			}

			if downloadErr != nil {
				job.Progress(-1, sourceEntry.Size, true)
				return xerrors.Errorf("failed to download %s to %s: %w", sourcePath, targetFilePath, downloadErr)
			}

			logger.Debugf("downloaded a data object %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceEntry.Size, sourceEntry.Size, false)

			if decryptionFlagValues.Decryption {
				logger.Debugf("decrypt a data object %s to %s", targetFilePath, decryptedTargetFilePath)
				err = encryptManager.DecryptFile(targetFilePath, decryptedTargetFilePath)
				if err != nil {
					return xerrors.Errorf("failed to decrypt %s: %w", targetFilePath, err)
				}

				logger.Debugf("removing a temp file %s", targetFilePath)
				os.Remove(targetFilePath)
			}

			if postTransferFlagValues.DeleteOnSuccess {
				logger.Debugf("removing source file %s", originalSourcePath)
				filesystem.RemoveFile(originalSourcePath, true)
			}

			return nil
		}

		if fileExist {
			// check transfer status file
			trxStatusFilePath := irodsclient_irodsfs.GetDataObjectTransferStatusFilePath(targetFilePath)
			trxStatusFileExist := false
			_, err = os.Stat(trxStatusFilePath)
			if err == nil {
				trxStatusFileExist = true
			}

			if trxStatusFileExist {
				// incomplete file - resume downloading
				fmt.Printf("resume downloading a data object %s\n", targetFilePath)
			} else if differentialTransferFlagValues.DifferentialTransfer {
				// trx status not exist
				if differentialTransferFlagValues.NoHash {
					if targetEntry.Size() == sourceEntry.Size {
						fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
						return nil
					}

					// delete file to not write to existing file
					os.Remove(targetFilePath)
				} else {
					if targetEntry.Size() == sourceEntry.Size {
						if len(sourceEntry.CheckSum) > 0 {
							// compare hash
							hash, err := irodsclient_util.HashLocalFile(targetFilePath, string(sourceEntry.CheckSumAlgorithm))
							if err != nil {
								return xerrors.Errorf("failed to get hash of %s: %w", targetFilePath, err)
							}

							if bytes.Equal(sourceEntry.CheckSum, hash) {
								fmt.Printf("skip downloading a data object %s. The file with the same hash already exists!\n", targetFilePath)
								return nil
							}
						}
					}

					// delete file to not write to existing file
					os.Remove(targetFilePath)
				}
			} else {
				if !forceFlagValues.Force {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
					if !overwrite {
						fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
						return nil
					}
				}

				// delete file to not write to existing file
				os.Remove(targetFilePath)
			}
		}

		threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
		parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a data object download %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		for _, entry := range entries {
			targetDirPath := targetPath
			if entry.Type != irodsclient_fs.FileEntry {
				// dir
				targetDirPath = commons.MakeTargetLocalFilePath(entry.Path, targetPath)
				err = os.MkdirAll(targetDirPath, 0766)
				if err != nil {
					return xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
				}
			}

			commons.MarkPathMap(inputPathMap, targetDirPath)

			err = getOne(parallelJobManager, inputPathMap, entry.Path, targetDirPath, forceFlagValues, parallelTransferFlagValues, differentialTransferFlagValues, decryptionFlagValues, postTransferFlagValues)
			if err != nil {
				return xerrors.Errorf("failed to perform get %s to %s: %w", entry.Path, targetDirPath, err)
			}
		}
	}
	return nil
}

func makeGetTargetDirPath(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, noRoot bool) (string, error) {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return "", xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := os.Stat(targetDirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", irodsclient_types.NewFileNotFoundError(targetDirPath)
			}

			return "", xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		return targetDirPath, nil
	} else {
		// dir
		_, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", irodsclient_types.NewFileNotFoundError(targetPath)
			}

			return "", xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

		targetDirPath := targetPath

		if !noRoot {
			// make target dir
			targetDirPath = commons.MakeTargetLocalFilePath(sourceEntry.Path, targetDirPath)
			err = os.MkdirAll(targetDirPath, 0766)
			if err != nil {
				return "", xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
			}
		}

		return targetDirPath, nil
	}
}

func getDeleteExtra(inputPathMap map[string]bool, targetPath string) error {
	targetPath = commons.MakeLocalPath(targetPath)

	return getDeleteExtraInternal(inputPathMap, targetPath)
}

func getDeleteExtraInternal(inputPathMap map[string]bool, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "getDeleteExtraInternal",
	})

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(targetPath)
		}

		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if !targetStat.IsDir() {
		// file
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra file %s", targetPath)
			removeErr := os.Remove(targetPath)
			if removeErr != nil {
				return removeErr
			}
		}
	} else {
		// dir
		if _, ok := inputPathMap[targetPath]; !ok {
			// extra dir
			logger.Debugf("removing an extra dir %s", targetPath)
			removeErr := os.RemoveAll(targetPath)
			if removeErr != nil {
				return removeErr
			}
		} else {
			// non extra dir
			entries, err := os.ReadDir(targetPath)
			if err != nil {
				return xerrors.Errorf("failed to list dir %s: %w", targetPath, err)
			}

			for _, entryInDir := range entries {
				newTargetPath := commons.MakeTargetLocalFilePath(entryInDir.Name(), targetPath)
				err = getDeleteExtraInternal(inputPathMap, newTargetPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

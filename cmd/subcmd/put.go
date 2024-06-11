package subcmd

import (
	"bytes"
	"fmt"
	"os"
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
	flag.SetCommonFlags(putCmd, false)

	flag.SetForceFlags(putCmd, false)
	flag.SetTicketAccessFlags(putCmd)
	flag.SetParallelTransferFlags(putCmd, true)
	flag.SetProgressFlags(putCmd)
	flag.SetRetryFlags(putCmd)
	flag.SetDifferentialTransferFlags(putCmd, true)
	flag.SetChecksumFlags(putCmd, true)
	flag.SetNoRootFlags(putCmd)
	flag.SetSyncFlags(putCmd)
	flag.SetEncryptionFlags(putCmd)
	flag.SetPostTransferFlagValues(putCmd)

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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
	checksumFlagValues := flag.GetChecksumFlagValues()
	noRootFlagValues := flag.GetNoRootFlagValues()
	syncFlagValues := flag.GetSyncFlagValues()
	encryptionFlagValues := flag.GetEncryptionFlagValues(command)
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

	// set default key for encryption
	if len(encryptionFlagValues.Key) == 0 {
		encryptionFlagValues.Key = account.Password
	}

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
		newTargetDirPath, err := makePutTargetDirPath(filesystem, sourcePath, targetPath, noRootFlagValues.NoRoot)
		if err != nil {
			return xerrors.Errorf("failed to make new target path for put %s to %s: %w", sourcePath, targetPath, err)
		}

		err = putOne(parallelJobManager, inputPathMap, sourcePath, newTargetDirPath, forceFlagValues, parallelTransferFlagValues, differentialTransferFlagValues, checksumFlagValues, encryptionFlagValues, postTransferFlagValues)
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

func getEncryptionManagerForEncrypt(encryptionFlagValues *flag.EncryptionFlagValues) *commons.EncryptionManager {
	manager := commons.NewEncryptionManager(encryptionFlagValues.Mode)

	switch encryptionFlagValues.Mode {
	case commons.EncryptionModeWinSCP, commons.EncryptionModePGP:
		manager.SetKey([]byte(encryptionFlagValues.Key))
	case commons.EncryptionModeSSH:
		manager.SetPublicPrivateKey(encryptionFlagValues.PublicPrivateKeyPath)
	}

	return manager
}

func putOne(parallelJobManager *commons.ParallelJobManager, inputPathMap map[string]bool, sourcePath string, targetPath string, forceFlagValues *flag.ForceFlagValues, parallelTransferFlagValues *flag.ParallelTransferFlagValues, differentialTransferFlagValues *flag.DifferentialTransferFlagValues, checksumFlagValues *flag.ChecksumFlagValues, encryptionFlagValues *flag.EncryptionFlagValues, postTransferFlagValues *flag.PostTransferFlagValues) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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

	// load encryption config from meta
	if !encryptionFlagValues.NoEncryption && !encryptionFlagValues.IgnoreMeta {
		targetDir := targetPath
		targetEntry, err := filesystem.Stat(targetPath)
		if err != nil {
			if irodsclient_types.IsFileNotFoundError(err) {
				// target path is file name
				targetDir = commons.GetDir(targetPath)
			} else {
				return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
			}
		} else {
			if !targetEntry.IsDir() {
				targetDir = commons.GetDir(targetPath)
			}
		}

		encryptionConfig := commons.GetEncryptionConfigFromMeta(filesystem, targetDir)
		if encryptionConfig.Required {
			encryptionFlagValues.Encryption = encryptionConfig.Required
			if encryptionConfig.Mode != commons.EncryptionModeUnknown {
				encryptionFlagValues.Mode = encryptionConfig.Mode
			}
		}
	}

	originalSourcePath := sourcePath

	if !sourceStat.IsDir() {
		// file
		// encrypt first if necessary
		if encryptionFlagValues.Encryption {
			logger.Debugf("encrypting file %s", sourcePath)

			encryptManager := getEncryptionManagerForEncrypt(encryptionFlagValues)
			newFilename, err := encryptManager.EncryptFilename(sourceStat.Name())
			if err != nil {
				return xerrors.Errorf("failed to encrypt %s: %w", sourcePath, err)
			}

			encryptedSourcePath := filepath.Join(encryptionFlagValues.TempPath, newFilename)
			err = encryptManager.EncryptFile(sourcePath, encryptedSourcePath)
			if err != nil {
				return xerrors.Errorf("failed to encrypt %s: %w", sourcePath, err)
			}

			encryptedSourceStat, err := os.Stat(encryptedSourcePath)
			if err != nil {
				return xerrors.Errorf("failed to stat file %s: %w", encryptedSourcePath, err)
			}

			sourcePath = encryptedSourcePath
			sourceStat = encryptedSourceStat
		}

		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		commons.MarkPathMap(inputPathMap, targetFilePath)

		fileExist := false
		targetEntry, err := filesystem.StatFile(targetFilePath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			fileExist = true
		}

		putTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackPut := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceStat.Size(), false)

			logger.Debugf("uploading a file %s to %s", sourcePath, targetFilePath)

			var uploadErr error

			// determine how to download

			if parallelTransferFlagValues.SingleTread || parallelTransferFlagValues.ThreadNumber == 1 {
				uploadErr = fs.UploadFile(sourcePath, targetFilePath, "", false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
			} else if parallelTransferFlagValues.RedirectToResource {
				uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
			} else if parallelTransferFlagValues.Icat {
				uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
			} else {
				// auto
				if sourceStat.Size() >= commons.RedirectToResourceMinSize {
					// redirect-to-resource
					uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
				} else {
					if filesystem.SupportParallelUpload() {
						uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
					} else {
						if sourceStat.Size() >= commons.ParallelUploadMinSize {
							// does not support parall upload via iCAT
							// redirect-to-resource
							uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
						} else {
							uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, checksumFlagValues.CalculateChecksum, checksumFlagValues.VerifyChecksum, callbackPut)
						}
					}
				}
			}

			if uploadErr != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to upload %s to %s: %w", sourcePath, targetFilePath, uploadErr)
			}

			logger.Debugf("uploaded a file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceStat.Size(), sourceStat.Size(), false)

			if encryptionFlagValues.Encryption {
				logger.Debugf("removing a temp file %s", sourcePath)
				os.Remove(sourcePath)
			}

			if postTransferFlagValues.DeleteOnSuccess {
				logger.Debugf("removing source file %s", originalSourcePath)
				os.Remove(originalSourcePath)
			}

			return nil
		}

		if fileExist {
			if differentialTransferFlagValues.DifferentialTransfer {
				if differentialTransferFlagValues.NoHash {
					if targetEntry.Size == sourceStat.Size() {
						commons.Printf("skip uploading a file %s. The file already exists!\n", targetFilePath)
						logger.Debugf("skip uploading a file %s. The file already exists!", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size == sourceStat.Size() {
						if len(targetEntry.CheckSum) > 0 {
							// compare hash
							hash, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm))
							if err != nil {
								return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
							}

							if bytes.Equal(hash, targetEntry.CheckSum) {
								commons.Printf("skip uploading a file %s. The file with the same hash already exists!\n", targetFilePath)
								logger.Debugf("skip uploading a file %s. The file with the same hash already exists!", targetFilePath)
								return nil
							}
						}
					}
				}
			} else {
				if !forceFlagValues.Force {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
					if !overwrite {
						commons.Printf("skip uploading a file %s. The data object already exists!\n", targetFilePath)
						logger.Debugf("skip uploading a file %s. The data object already exists!", targetFilePath)
						return nil
					}
				}
			}
		}

		threadsRequired := computeThreadsRequiredForPut(filesystem, parallelTransferFlagValues.SingleTread, sourceStat.Size())
		parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a local file upload %s to %s", sourcePath, targetFilePath)
	} else {
		logger.Debugf("uploading a local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		for _, entry := range entries {
			encryptionFlagValuesCopy := encryptionFlagValues

			targetDirPath := targetPath
			if entry.IsDir() {
				// dir
				targetDirPath = commons.MakeTargetIRODSFilePath(filesystem, entry.Name(), targetPath)

				if !filesystem.ExistsDir(targetDirPath) {
					// not exist
					err = filesystem.MakeDir(targetDirPath, true)
					if err != nil {
						return xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
					}
				}
			}

			commons.MarkPathMap(inputPathMap, targetDirPath)

			newSourcePath := filepath.Join(sourcePath, entry.Name())
			err = putOne(parallelJobManager, inputPathMap, newSourcePath, targetDirPath, forceFlagValues, parallelTransferFlagValues, differentialTransferFlagValues, checksumFlagValues, encryptionFlagValuesCopy, postTransferFlagValues)
			if err != nil {
				return xerrors.Errorf("failed to perform put %s to %s: %w", newSourcePath, targetDirPath, err)
			}
		}
	}
	return nil
}

func makePutTargetDirPath(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, noRoot bool) (string, error) {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return "", xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
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
		_, err := filesystem.Stat(targetPath)
		if err != nil {
			return "", xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

		targetDirPath := targetPath

		if !noRoot {
			// make target dir
			targetDirPath = commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
			err = filesystem.MakeDir(targetDirPath, true)
			if err != nil {
				return "", xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
			}
		}

		return targetDirPath, nil
	}
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
		"package":  "subcmd",
		"function": "putDeleteExtraInternal",
	})

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
		// file
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

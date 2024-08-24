package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

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
	flag.SetRecursiveFlags(getCmd, true)
	flag.SetTicketAccessFlags(getCmd)
	flag.SetParallelTransferFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetDifferentialTransferFlags(getCmd, true)
	flag.SetChecksumFlags(getCmd, false)
	flag.SetTransferReportFlags(getCmd)
	flag.SetNoRootFlags(getCmd)
	flag.SetSyncFlags(getCmd, false)
	flag.SetDecryptionFlags(getCmd)
	flag.SetHiddenFileFlags(getCmd)
	flag.SetPostTransferFlagValues(getCmd)

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	get, err := NewGetCommand(command, args)
	if err != nil {
		return err
	}

	return get.Process()
}

type GetCommand struct {
	command *cobra.Command

	forceFlagValues                *flag.ForceFlagValues
	ticketAccessFlagValues         *flag.TicketAccessFlagValues
	parallelTransferFlagValues     *flag.ParallelTransferFlagValues
	progressFlagValues             *flag.ProgressFlagValues
	retryFlagValues                *flag.RetryFlagValues
	differentialTransferFlagValues *flag.DifferentialTransferFlagValues
	checksumFlagValues             *flag.ChecksumFlagValues
	noRootFlagValues               *flag.NoRootFlagValues
	syncFlagValues                 *flag.SyncFlagValues
	decryptionFlagValues           *flag.DecryptionFlagValues
	postTransferFlagValues         *flag.PostTransferFlagValues
	hiddenFileFlagValues           *flag.HiddenFileFlagValues
	transferReportFlagValues       *flag.TransferReportFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string

	parallelJobManager    *commons.ParallelJobManager
	transferReportManager *commons.TransferReportManager
	updatedPathMap        map[string]bool
}

func NewGetCommand(command *cobra.Command, args []string) (*GetCommand, error) {
	get := &GetCommand{
		command: command,

		forceFlagValues:                flag.GetForceFlagValues(),
		ticketAccessFlagValues:         flag.GetTicketAccessFlagValues(),
		parallelTransferFlagValues:     flag.GetParallelTransferFlagValues(),
		progressFlagValues:             flag.GetProgressFlagValues(),
		retryFlagValues:                flag.GetRetryFlagValues(),
		differentialTransferFlagValues: flag.GetDifferentialTransferFlagValues(),
		checksumFlagValues:             flag.GetChecksumFlagValues(),
		noRootFlagValues:               flag.GetNoRootFlagValues(),
		syncFlagValues:                 flag.GetSyncFlagValues(),
		decryptionFlagValues:           flag.GetDecryptionFlagValues(command),
		postTransferFlagValues:         flag.GetPostTransferFlagValues(),
		hiddenFileFlagValues:           flag.GetHiddenFileFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	get.maxConnectionNum = get.parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	// path
	get.targetPath = "./"
	get.sourcePaths = args

	if len(args) >= 2 {
		get.targetPath = args[len(args)-1]
		get.sourcePaths = args[:len(args)-1]
	}

	if get.noRootFlagValues.NoRoot && len(get.sourcePaths) > 1 {
		return nil, xerrors.Errorf("failed to get multiple source collections without creating root directory")
	}

	return get, nil
}

func (get *GetCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(get.command)
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

	// config
	appConfig := commons.GetConfig()
	syncAccount := false
	if len(get.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket %q", get.ticketAccessFlagValues.Name)
		appConfig.Ticket = get.ticketAccessFlagValues.Name
		syncAccount = true
	}

	if syncAccount {
		err := commons.SyncAccount()
		if err != nil {
			return err
		}
	}

	// handle retry
	if get.retryFlagValues.RetryNumber > 0 && !get.retryFlagValues.RetryChild {
		err := commons.RunWithRetry(get.retryFlagValues.RetryNumber, get.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", get.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	get.account = commons.GetAccount()
	get.filesystem, err = commons.GetIRODSFSClientAdvanced(get.account, get.maxConnectionNum, get.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer get.filesystem.Release()

	// transfer report
	get.transferReportManager, err = commons.NewTransferReportManager(get.transferReportFlagValues.Report, get.transferReportFlagValues.ReportPath, get.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer get.transferReportManager.Release()

	// set default key for decryption
	if len(get.decryptionFlagValues.Key) == 0 {
		get.decryptionFlagValues.Key = get.account.Password
	}

	// parallel job manager
	get.parallelJobManager = commons.NewParallelJobManager(get.filesystem, get.parallelTransferFlagValues.ThreadNumber, get.progressFlagValues.ShowProgress, get.progressFlagValues.ShowFullPath)
	get.parallelJobManager.Start()

	// run
	if len(get.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = get.ensureTargetIsDir(get.targetPath)
		if err != nil {
			return err
		}
	}

	for _, sourcePath := range get.sourcePaths {
		err = get.getOne(sourcePath, get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get %q to %q: %w", sourcePath, get.targetPath, err)
		}
	}

	get.parallelJobManager.DoneScheduling()
	err = get.parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// delete on success
	if get.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range get.sourcePaths {
			logger.Infof("deleting source %q after successful data get", sourcePath)

			err := get.deleteOnSuccess(sourcePath)
			if err != nil {
				return xerrors.Errorf("failed to delete source %q: %w", sourcePath, err)
			}
		}
	}

	// delete extra
	if get.syncFlagValues.Delete {
		logger.Infof("deleting extra files and directories under %q", get.targetPath)

		err := get.deleteExtra(get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func (get *GetCommand) ensureTargetIsDir(targetPath string) error {
	targetPath = commons.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// not exist
			return commons.NewNotDirError(targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetStat.IsDir() {
		return commons.NewNotDirError(targetPath)
	}

	return nil
}

func (get *GetCommand) requireDecryption(sourcePath string) bool {
	if get.decryptionFlagValues.NoDecryption {
		return false
	}

	if !get.decryptionFlagValues.Decryption {
		return false
	}

	mode := commons.DetectEncryptionMode(sourcePath)
	return mode != commons.EncryptionModeUnknown
}

func (get *GetCommand) hasTransferStatusFile(targetPath string) bool {
	// check transfer status file
	trxStatusFilePath := irodsclient_irodsfs.GetDataObjectTransferStatusFilePath(targetPath)
	_, err := os.Stat(trxStatusFilePath)
	return err == nil
}

func (get *GetCommand) getOne(sourcePath string, targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		// dir
		if !get.noRootFlagValues.NoRoot {
			targetPath = commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		}

		return get.getDir(sourceEntry, targetPath)
	}

	// file
	if get.requireDecryption(sourceEntry.Path) {
		// decrypt filename
		tempPath, newTargetPath, err := get.getPathsForDecryption(sourceEntry.Path, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get decryption path for %q: %w", sourceEntry.Path, err)
		}

		return get.getFile(sourceEntry, tempPath, newTargetPath)
	}

	targetPath = commons.MakeTargetLocalFilePath(sourcePath, targetPath)
	return get.getFile(sourceEntry, "", targetPath)
}

func (get *GetCommand) scheduleGet(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string, resume bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "scheduleGet",
	})

	getTask := func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		callbackGet := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, sourceEntry.Size, false)

		logger.Debugf("downloading a data object %q to %q", sourceEntry.Path, targetPath)

		var downloadErr error
		var downloadResult *irodsclient_fs.FileTransferResult
		notes := []string{}

		downloadPath := targetPath
		if len(tempPath) > 0 {
			downloadPath = tempPath
		}

		// determine how to download
		if get.parallelTransferFlagValues.SingleTread || get.parallelTransferFlagValues.ThreadNumber == 1 {
			downloadResult, downloadErr = fs.DownloadFileResumable(sourceEntry.Path, "", downloadPath, get.checksumFlagValues.VerifyChecksum, callbackGet)
			notes = append(notes, "icat", "single-thread")
		} else if get.parallelTransferFlagValues.RedirectToResource {
			if resume {
				downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
				notes = append(notes, "icat", "multi-thread", "resume")
			} else {
				downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
				notes = append(notes, "redirect-to-resource")
			}
		} else if get.parallelTransferFlagValues.Icat {
			downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
			notes = append(notes, "icat", "multi-thread")
		} else {
			// auto
			if sourceEntry.Size >= commons.RedirectToResourceMinSize {
				// redirect-to-resource
				if resume {
					downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
					notes = append(notes, "icat", "multi-thread", "resume")
				} else {
					downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
					notes = append(notes, "redirect-to-resource")
				}
			} else {
				downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
				notes = append(notes, "icat", "multi-thread")
			}
		}

		if downloadErr != nil {
			job.Progress(-1, sourceEntry.Size, true)
			return xerrors.Errorf("failed to download %q to %q: %w", sourceEntry.Path, targetPath, downloadErr)
		}

		// decrypt
		if get.requireDecryption(sourceEntry.Path) {
			decrypted, err := get.decryptFile(sourceEntry.Path, tempPath, targetPath)
			if err != nil {
				job.Progress(-1, sourceEntry.Size, true)
				return xerrors.Errorf("failed to decrypt file: %w", err)
			}

			if decrypted {
				notes = append(notes, "decrypted", targetPath)
			}
		}

		err := get.transferReportManager.AddTransfer(downloadResult, commons.TransferMethodGet, downloadErr, notes)
		if err != nil {
			job.Progress(-1, sourceEntry.Size, true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}

		logger.Debugf("downloaded a data object %q to %q", sourceEntry.Path, targetPath)
		job.Progress(sourceEntry.Size, sourceEntry.Size, false)

		job.Done()
		return nil
	}

	threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
	err := get.parallelJobManager.Schedule(sourceEntry.Path, getTask, threadsRequired, progress.UnitsBytes)
	if err != nil {
		return xerrors.Errorf("failed to schedule download %q to %q: %w", sourceEntry.Path, targetPath, err)
	}

	logger.Debugf("scheduled a data object download %q to %q", sourceEntry.Path, targetPath)

	return nil
}

func (get *GetCommand) getFile(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "getFile",
	})

	commons.MarkPathMap(get.updatedPathMap, targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a file with new name
			return get.scheduleGet(sourceEntry, tempPath, targetPath, false)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetStat.IsDir() {
		if get.syncFlagValues.Sync {
			// if it is sync, remove
			if get.forceFlagValues.Force {
				removeErr := os.RemoveAll(targetPath)

				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodDelete,
					StartAt:    now,
					EndAt:      now,
					SourcePath: targetPath,
					Error:      removeErr,
					Notes:      []string{"overwrite", "get", "dir"},
				}

				get.transferReportManager.AddFile(reportFile)

				if removeErr != nil {
					return removeErr
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("overwriting a file %q, but directory exists. Overwrite?", targetPath))
				if overwrite {
					removeErr := os.RemoveAll(targetPath)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "get", "dir"},
					}

					get.transferReportManager.AddFile(reportFile)

					if removeErr != nil {
						return removeErr
					}
				} else {
					return commons.NewNotFileError(targetPath)
				}
			}
		} else {
			return commons.NewNotFileError(targetPath)
		}
	}

	// check transfer status file
	if get.hasTransferStatusFile(targetPath) {
		// incomplete file - resume downloading
		commons.Printf("resume downloading a data object %q\n", targetPath)
		logger.Debugf("resume downloading a data object %q", targetPath)

		return get.scheduleGet(sourceEntry, tempPath, targetPath, true)
	}

	if get.differentialTransferFlagValues.DifferentialTransfer {
		if get.differentialTransferFlagValues.NoHash {
			if targetStat.Size() == sourceEntry.Size {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:         commons.TransferMethodGet,
					StartAt:        now,
					EndAt:          now,
					SourcePath:     sourceEntry.Path,
					SourceSize:     sourceEntry.Size,
					SourceChecksum: hex.EncodeToString(sourceEntry.CheckSum),

					DestPath:          targetPath,
					DestSize:          targetStat.Size(),
					ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					Notes:             []string{"differential", "no_hash", "same file size", "skip"},
				}

				get.transferReportManager.AddFile(reportFile)

				commons.Printf("skip downloading a data object %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip downloading a data object %q to %q. The file already exists!", sourceEntry.Path, targetPath)
				return nil
			}
		} else {
			if targetStat.Size() == sourceEntry.Size {
				// compare hash
				if len(sourceEntry.CheckSum) > 0 {
					localChecksum, err := irodsclient_util.HashLocalFile(targetPath, string(sourceEntry.CheckSumAlgorithm))
					if err != nil {
						return xerrors.Errorf("failed to get hash of %q: %w", targetPath, err)
					}

					if bytes.Equal(sourceEntry.CheckSum, localChecksum) {
						// skip
						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:            commons.TransferMethodGet,
							StartAt:           now,
							EndAt:             now,
							SourcePath:        sourceEntry.Path,
							SourceSize:        sourceEntry.Size,
							SourceChecksum:    hex.EncodeToString(sourceEntry.CheckSum),
							DestPath:          targetPath,
							DestSize:          targetStat.Size(),
							DestChecksum:      hex.EncodeToString(localChecksum),
							ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
							Notes:             []string{"differential", "same checksum", "skip"},
						}

						get.transferReportManager.AddFile(reportFile)

						commons.Printf("skip downloading a data object %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
						logger.Debugf("skip downloading a data object %q to %q. The file with the same hash already exists!", sourceEntry.Path, targetPath)
						return nil
					}
				}
			}
		}
	} else {
		if !get.forceFlagValues.Force {
			// ask
			overwrite := commons.InputYN(fmt.Sprintf("file %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:            commons.TransferMethodGet,
					StartAt:           now,
					EndAt:             now,
					SourcePath:        sourceEntry.Path,
					SourceSize:        sourceEntry.Size,
					SourceChecksum:    hex.EncodeToString(sourceEntry.CheckSum),
					DestPath:          targetPath,
					DestSize:          targetStat.Size(),
					ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					Notes:             []string{"no_overwrite", "skip"},
				}

				get.transferReportManager.AddFile(reportFile)

				commons.Printf("skip downloading a data object %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip downloading a data object %q to %q. The file already exists!", sourceEntry.Path, targetPath)
				return nil
			}
		}
	}

	// schedule
	return get.scheduleGet(sourceEntry, tempPath, targetPath, false)
}

func (get *GetCommand) getDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	commons.MarkPathMap(get.updatedPathMap, targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a directorywith new name
			err = os.MkdirAll(targetPath, 0766)
			if err != nil {
				return xerrors.Errorf("failed to make a directory %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodGet,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			get.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetStat.IsDir() {
			if get.syncFlagValues.Sync {
				// if it is sync, remove
				if get.forceFlagValues.Force {
					removeErr := os.Remove(targetPath)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "get"},
					}

					get.transferReportManager.AddFile(reportFile)

					if removeErr != nil {
						return removeErr
					}
				} else {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						removeErr := os.Remove(targetPath)

						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:     commons.TransferMethodDelete,
							StartAt:    now,
							EndAt:      now,
							SourcePath: targetPath,
							Error:      removeErr,
							Notes:      []string{"overwrite", "put"},
						}

						get.transferReportManager.AddFile(reportFile)

						if removeErr != nil {
							return removeErr
						}
					} else {
						return commons.NewNotDirError(targetPath)
					}
				}
			} else {
				return commons.NewNotDirError(targetPath)
			}
		}
	}

	// load encryption config
	requireDecryption := get.requireDecryption(sourceEntry.Path)

	// get entries
	entries, err := get.filesystem.List(sourceEntry.Path)
	if err != nil {
		return xerrors.Errorf("failed to list a directory %q: %w", sourceEntry.Path, err)
	}

	for _, entry := range entries {
		if get.hiddenFileFlagValues.Exclude {
			if strings.HasPrefix(entry.Name, ".") {
				continue
			}
		}

		newEntryPath := commons.MakeTargetLocalFilePath(entry.Path, targetPath)

		if entry.IsDir() {
			// dir
			err = get.getDir(entry, newEntryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			if requireDecryption {
				// decrypt filename
				tempPath, newTargetPath, err := get.getPathsForDecryption(entry.Path, targetPath)
				if err != nil {
					return xerrors.Errorf("failed to get decryption path for %q: %w", entry.Path, err)
				}

				err = get.getFile(entry, tempPath, newTargetPath)
				if err != nil {
					return err
				}
			} else {
				err = get.getFile(entry, "", newEntryPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (get *GetCommand) deleteOnSuccess(sourcePath string) error {
	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		return get.filesystem.RemoveDir(sourcePath, true, true)
	}

	return get.filesystem.RemoveFile(sourcePath, true)
}

func (get *GetCommand) deleteExtra(targetPath string) error {
	targetPath = commons.MakeLocalPath(targetPath)

	return get.deleteExtraInternal(targetPath)
}

func (get *GetCommand) deleteExtraInternal(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "deleteExtraInternal",
	})

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target is file
	if !targetStat.IsDir() {
		if _, ok := get.updatedPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra file %q", targetPath)

			removeErr := os.Remove(targetPath)

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodDelete,
				StartAt:    now,
				EndAt:      now,
				SourcePath: targetPath,
				Error:      removeErr,
				Notes:      []string{"extra", "get"},
			}

			get.transferReportManager.AddFile(reportFile)

			if removeErr != nil {
				return removeErr
			}
		}

		return nil
	}

	// target is dir
	if _, ok := get.updatedPathMap[targetPath]; !ok {
		// extra dir
		logger.Debugf("removing an extra directory %q", targetPath)

		removeErr := os.RemoveAll(targetPath)

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:     commons.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: targetPath,
			Error:      removeErr,
			Notes:      []string{"extra", "get", "dir"},
		}

		get.transferReportManager.AddFile(reportFile)

		if removeErr != nil {
			return removeErr
		}
	} else {
		// non extra dir
		// scan recursively
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to list a directory %q: %w", targetPath, err)
		}

		for _, entry := range entries {
			newTargetPath := path.Join(targetPath, entry.Name())
			err = get.deleteExtraInternal(newTargetPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (get *GetCommand) getEncryptionManagerForDecryption(mode commons.EncryptionMode) *commons.EncryptionManager {
	manager := commons.NewEncryptionManager(mode)

	switch mode {
	case commons.EncryptionModeWinSCP, commons.EncryptionModePGP:
		manager.SetKey([]byte(get.decryptionFlagValues.Key))
	case commons.EncryptionModeSSH:
		manager.SetPublicPrivateKey(get.decryptionFlagValues.PrivateKeyPath)
	}

	return manager
}

func (get *GetCommand) getPathsForDecryption(sourcePath string, targetPath string) (string, string, error) {
	encryptionMode := commons.DetectEncryptionMode(sourcePath)

	if encryptionMode != commons.EncryptionModeUnknown {
		// encrypted file
		sourceFilename := commons.GetBasename(sourcePath)
		encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

		tempFilePath := commons.MakeTargetLocalFilePath(sourcePath, get.decryptionFlagValues.TempPath)

		decryptedFilename, err := encryptManager.DecryptFilename(sourceFilename)
		if err != nil {
			return "", "", xerrors.Errorf("failed to decrypt filename %q: %w", sourcePath, err)
		}

		targetFilePath := commons.MakeTargetLocalFilePath(decryptedFilename, targetPath)

		return tempFilePath, targetFilePath, nil
	}

	targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)

	return "", targetFilePath, nil
}

func (get *GetCommand) decryptFile(sourcePath string, encryptedFilePath string, targetPath string) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "decryptFile",
	})

	encryptionMode := commons.DetectEncryptionMode(sourcePath)

	if encryptionMode != commons.EncryptionModeUnknown {
		logger.Debugf("decrypt a data object %q to %q", encryptedFilePath, targetPath)

		encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

		err := encryptManager.DecryptFile(encryptedFilePath, targetPath)
		if err != nil {
			return false, xerrors.Errorf("failed to decrypt %q to %q: %w", encryptedFilePath, targetPath, err)
		}

		logger.Debugf("removing a temp file %q", encryptedFilePath)
		os.Remove(encryptedFilePath)

		return true, nil
	}

	return false, nil
}

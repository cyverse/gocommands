package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	Use:     "put <local-file-or-dir>... <dest-data-object-or-collection>",
	Aliases: []string{"iput", "upload"},
	Short:   "Upload files or directories to an iRODS data-object or collection",
	Long:    `This command uploads files or directories to the specified iRODS data-object or collection.`,
	RunE:    processPutCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(putCmd, false)

	flag.SetBundleTransferFlags(putCmd, true, true)
	flag.SetParallelTransferFlags(putCmd, false, false)
	flag.SetForceFlags(putCmd, false)
	flag.SetRecursiveFlags(putCmd, true)
	flag.SetTicketAccessFlags(putCmd)
	flag.SetProgressFlags(putCmd)
	flag.SetRetryFlags(putCmd)
	flag.SetDifferentialTransferFlags(putCmd, false)
	flag.SetChecksumFlags(putCmd, false, false)
	flag.SetNoRootFlags(putCmd)
	flag.SetSyncFlags(putCmd, true)
	flag.SetEncryptionFlags(putCmd)
	flag.SetHiddenFileFlags(putCmd)
	flag.SetPostTransferFlagValues(putCmd)
	flag.SetTransferReportFlags(putCmd)

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
	put, err := NewPutCommand(command, args)
	if err != nil {
		return err
	}

	return put.Process()
}

type PutCommand struct {
	command *cobra.Command

	commonFlagValues               *flag.CommonFlagValues
	bundleTransferFlagValues       *flag.BundleTransferFlagValues
	parallelTransferFlagValues     *flag.ParallelTransferFlagValues
	forceFlagValues                *flag.ForceFlagValues
	recursiveFlagValues            *flag.RecursiveFlagValues
	ticketAccessFlagValues         *flag.TicketAccessFlagValues
	progressFlagValues             *flag.ProgressFlagValues
	retryFlagValues                *flag.RetryFlagValues
	differentialTransferFlagValues *flag.DifferentialTransferFlagValues
	checksumFlagValues             *flag.ChecksumFlagValues
	noRootFlagValues               *flag.NoRootFlagValues
	syncFlagValues                 *flag.SyncFlagValues
	encryptionFlagValues           *flag.EncryptionFlagValues
	hiddenFileFlagValues           *flag.HiddenFileFlagValues
	postTransferFlagValues         *flag.PostTransferFlagValues
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

func NewPutCommand(command *cobra.Command, args []string) (*PutCommand, error) {
	put := &PutCommand{
		command: command,

		commonFlagValues:               flag.GetCommonFlagValues(command),
		bundleTransferFlagValues:       flag.GetBundleTransferFlagValues(),
		parallelTransferFlagValues:     flag.GetParallelTransferFlagValues(),
		forceFlagValues:                flag.GetForceFlagValues(),
		recursiveFlagValues:            flag.GetRecursiveFlagValues(),
		ticketAccessFlagValues:         flag.GetTicketAccessFlagValues(),
		progressFlagValues:             flag.GetProgressFlagValues(),
		retryFlagValues:                flag.GetRetryFlagValues(),
		differentialTransferFlagValues: flag.GetDifferentialTransferFlagValues(),
		checksumFlagValues:             flag.GetChecksumFlagValues(),
		noRootFlagValues:               flag.GetNoRootFlagValues(),
		syncFlagValues:                 flag.GetSyncFlagValues(),
		encryptionFlagValues:           flag.GetEncryptionFlagValues(command),
		hiddenFileFlagValues:           flag.GetHiddenFileFlagValues(),
		postTransferFlagValues:         flag.GetPostTransferFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	put.maxConnectionNum = put.parallelTransferFlagValues.ThreadNumber

	// path
	put.targetPath = "./"
	put.sourcePaths = args

	if len(args) >= 2 {
		put.targetPath = args[len(args)-1]
		put.sourcePaths = args[:len(args)-1]
	}

	if put.noRootFlagValues.NoRoot && len(put.sourcePaths) > 1 {
		return nil, xerrors.Errorf("failed to put multiple source collections without creating root directory")
	}

	return put, nil
}

func (put *PutCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(put.command)
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

	// handle retry
	if put.retryFlagValues.RetryNumber > 0 && !put.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(put.retryFlagValues.RetryNumber, put.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", put.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	put.account = commons.GetSessionConfig().ToIRODSAccount()
	if len(put.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", put.ticketAccessFlagValues.Name)
		put.account.Ticket = put.ticketAccessFlagValues.Name
	}

	put.filesystem, err = commons.GetIRODSFSClientForLargeFileIO(put.account, put.maxConnectionNum, put.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer put.filesystem.Release()

	// transfer report
	put.transferReportManager, err = commons.NewTransferReportManager(put.transferReportFlagValues.Report, put.transferReportFlagValues.ReportPath, put.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer put.transferReportManager.Release()

	// set default key for encryption
	if len(put.encryptionFlagValues.Key) == 0 {
		put.encryptionFlagValues.Key = put.account.Password
	}

	// parallel job manager
	put.parallelJobManager = commons.NewParallelJobManager(put.filesystem, put.parallelTransferFlagValues.ThreadNumber, put.progressFlagValues.ShowProgress, put.progressFlagValues.ShowFullPath)
	put.parallelJobManager.Start()

	// run
	if len(put.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = put.ensureTargetIsDir(put.targetPath)
		if err != nil {
			return err
		}
	}

	for _, sourcePath := range put.sourcePaths {
		err = put.putOne(sourcePath, put.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to put %q to %q: %w", sourcePath, put.targetPath, err)
		}
	}

	put.parallelJobManager.DoneScheduling()
	err = put.parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// delete on success
	if put.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range put.sourcePaths {
			logger.Infof("deleting source %q after successful data put", sourcePath)

			err := put.deleteOnSuccess(sourcePath)
			if err != nil {
				return xerrors.Errorf("failed to delete source %q: %w", sourcePath, err)
			}
		}
	}

	// delete extra
	if put.syncFlagValues.Delete {
		logger.Infof("deleting extra files and directories under %q", put.targetPath)

		err = put.deleteExtra(put.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func (put *PutCommand) ensureTargetIsDir(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := put.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// not exist
			return commons.NewNotDirError(targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		return commons.NewNotDirError(targetPath)
	}

	return nil
}

func (put *PutCommand) requireEncryption(targetPath string, parentEncryption bool, parentEncryptionMode commons.EncryptionMode) (bool, commons.EncryptionMode) {
	if put.encryptionFlagValues.Encryption {
		return true, put.encryptionFlagValues.Mode
	}

	if put.encryptionFlagValues.NoEncryption {
		return false, commons.EncryptionModeUnknown
	}

	if !put.encryptionFlagValues.IgnoreMeta {
		// load encryption config from meta
		targetDir := targetPath

		targetEntry, err := put.filesystem.Stat(targetPath)
		if err != nil {
			if irodsclient_types.IsFileNotFoundError(err) {
				targetDir = commons.GetDir(targetPath)
			} else {
				return parentEncryption, parentEncryptionMode
			}
		} else {
			if !targetEntry.IsDir() {
				targetDir = commons.GetDir(targetEntry.Path)
			}
		}

		encryptionConfig := commons.GetEncryptionConfigFromMeta(put.filesystem, targetDir)

		if encryptionConfig.Mode == commons.EncryptionModeUnknown {
			if put.encryptionFlagValues.Mode == commons.EncryptionModeUnknown {
				return false, commons.EncryptionModeUnknown
			}

			return encryptionConfig.Required, put.encryptionFlagValues.Mode
		}

		return encryptionConfig.Required, encryptionConfig.Mode
	}

	return parentEncryption, parentEncryptionMode
}

func (put *PutCommand) putOne(sourcePath string, targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := put.account.ClientZone
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceStat.IsDir() {
		// dir
		if !put.noRootFlagValues.NoRoot {
			targetPath = commons.MakeTargetIRODSFilePath(put.filesystem, sourcePath, targetPath)
		}

		return put.putDir(sourceStat, sourcePath, targetPath, false, commons.EncryptionModeUnknown)
	}

	// file
	requireEncryption, encryptionMode := put.requireEncryption(targetPath, false, commons.EncryptionModeUnknown)
	if requireEncryption {
		// encrypt filename
		tempPath, newTargetPath, err := put.getPathsForEncryption(sourcePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get encryption path for %q: %w", sourcePath, err)
		}

		return put.putFile(sourceStat, sourcePath, tempPath, newTargetPath, requireEncryption, encryptionMode)
	}

	targetPath = commons.MakeTargetIRODSFilePath(put.filesystem, sourcePath, targetPath)
	return put.putFile(sourceStat, sourcePath, "", targetPath, requireEncryption, commons.EncryptionModeUnknown)
}

func (put *PutCommand) schedulePut(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, requireDecryption bool, encryptionMode commons.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "schedulePut",
	})

	threadsRequired := put.calculateThreadForTransferJob(sourceStat.Size())

	putTask := func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		callbackPut := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, sourceStat.Size(), false)

		logger.Debugf("uploading a file %q to %q", sourcePath, targetPath)

		var uploadErr error
		var uploadResult *irodsclient_fs.FileTransferResult
		notes := []string{}

		// encrypt
		if requireDecryption {
			encrypted, err := put.encryptFile(sourcePath, tempPath, encryptionMode)
			if err != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to decrypt file: %w", err)
			}

			if encrypted {
				notes = append(notes, "encrypted", targetPath)
			}
		}

		uploadSourcePath := sourcePath
		if len(tempPath) > 0 {
			uploadSourcePath = tempPath
		}

		parentTargetPath := commons.GetDir(targetPath)
		if !fs.ExistsDir(parentTargetPath) {
			err := fs.MakeDir(parentTargetPath, true)
			if err != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to make a collection %q: %w", parentTargetPath, err)
			}
		}

		// determine how to upload
		transferMode := put.determineTransferMode(sourceStat.Size())
		switch transferMode {
		case commons.TransferModeRedirect:
			uploadResult, uploadErr = fs.UploadFileRedirectToResource(uploadSourcePath, targetPath, "", threadsRequired, false, put.checksumFlagValues.CalculateChecksum, put.checksumFlagValues.VerifyChecksum, false, callbackPut)
			notes = append(notes, "redirect-to-resource")
		case commons.TransferModeSingleThread:
			uploadResult, uploadErr = fs.UploadFile(uploadSourcePath, targetPath, "", false, put.checksumFlagValues.CalculateChecksum, put.checksumFlagValues.VerifyChecksum, false, callbackPut)
			notes = append(notes, "icat", "single-thread")
		case commons.TransferModeICAT:
			fallthrough
		default:
			uploadResult, uploadErr = fs.UploadFileParallel(uploadSourcePath, targetPath, "", threadsRequired, false, put.checksumFlagValues.CalculateChecksum, put.checksumFlagValues.VerifyChecksum, false, callbackPut)
			notes = append(notes, "icat", "multi-thread")
		}

		if uploadErr != nil {
			job.Progress(-1, sourceStat.Size(), true)
			return xerrors.Errorf("failed to upload %q to %q: %w", sourcePath, targetPath, uploadErr)
		}

		err := put.transferReportManager.AddTransfer(uploadResult, commons.TransferMethodPut, uploadErr, notes)
		if err != nil {
			job.Progress(-1, sourceStat.Size(), true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}

		if requireDecryption {
			logger.Debugf("removing a temp file %q", tempPath)
			os.Remove(tempPath)
		}

		logger.Debugf("uploaded a file %q to %q", sourcePath, targetPath)
		job.Progress(sourceStat.Size(), sourceStat.Size(), false)

		job.Done()
		return nil
	}

	err := put.parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
	if err != nil {
		return xerrors.Errorf("failed to schedule upload %q to %q: %w", sourcePath, targetPath, err)
	}

	logger.Debugf("scheduled a file upload %q to %q, %d threads", sourcePath, targetPath, threadsRequired)

	return nil
}

func (put *PutCommand) putFile(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, requireEncryption bool, encryptionMode commons.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "putFile",
	})

	commons.MarkIRODSPathMap(put.updatedPathMap, targetPath)

	if put.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			put.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a file %q to %q. The file is hidden!\n", sourcePath, targetPath)
			logger.Debugf("skip uploading a file %q to %q. The file is hidden!", sourcePath, targetPath)
			return nil
		}
	}

	if put.syncFlagValues.Age > 0 {
		// exclude old
		age := time.Since(sourceStat.ModTime())
		maxAge := time.Duration(put.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"age", "skip"},
			}

			put.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a file %q to %q. The file is too old (%s > %s)!\n", sourcePath, targetPath, age, maxAge)
			logger.Debugf("skip uploading a file %q to %q. The file is too old (%s > %s)!", sourcePath, targetPath, age, maxAge)
			return nil
		}
	}

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			return put.schedulePut(sourceStat, sourcePath, tempPath, targetPath, requireEncryption, encryptionMode)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if put.syncFlagValues.Sync {
			// if it is sync, remove
			if put.forceFlagValues.Force {
				removeErr := put.filesystem.RemoveDir(targetPath, true, true)

				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodDelete,
					StartAt:    now,
					EndAt:      now,
					SourcePath: targetPath,
					Error:      removeErr,
					Notes:      []string{"overwrite", "put", "dir"},
				}

				put.transferReportManager.AddFile(reportFile)

				if removeErr != nil {
					return removeErr
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("overwriting a file %q, but directory exists. Overwrite?", targetPath))
				if overwrite {
					removeErr := put.filesystem.RemoveDir(targetPath, true, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "put", "dir"},
					}

					put.transferReportManager.AddFile(reportFile)

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

	if put.differentialTransferFlagValues.DifferentialTransfer {
		if put.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceStat.Size() {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodPut,
					StartAt:    now,
					EndAt:      now,
					SourcePath: sourcePath,
					SourceSize: sourceStat.Size(),

					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),
					Notes:                 []string{"differential", "no_hash", "same file size", "skip"},
				}

				put.transferReportManager.AddFile(reportFile)

				commons.Printf("skip uploading a file %q to %q. The file already exists!\n", sourcePath, targetPath)
				logger.Debugf("skip uploading a file %q to %q. The file already exists!", sourcePath, targetPath)
				return nil
			}
		} else {
			if targetEntry.Size == sourceStat.Size() {
				// compare hash
				if len(targetEntry.CheckSum) > 0 {
					localChecksum, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm))
					if err != nil {
						return xerrors.Errorf("failed to get hash for %q: %w", sourcePath, err)
					}

					if bytes.Equal(localChecksum, targetEntry.CheckSum) {
						// skip
						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:                  commons.TransferMethodPut,
							StartAt:                 now,
							EndAt:                   now,
							SourcePath:              sourcePath,
							SourceSize:              sourceStat.Size(),
							SourceChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),
							SourceChecksum:          hex.EncodeToString(localChecksum),
							DestPath:                targetEntry.Path,
							DestSize:                targetEntry.Size,
							DestChecksum:            hex.EncodeToString(targetEntry.CheckSum),
							DestChecksumAlgorithm:   string(targetEntry.CheckSumAlgorithm),
							Notes:                   []string{"differential", "same checksum", "skip"},
						}

						put.transferReportManager.AddFile(reportFile)

						commons.Printf("skip uploading a file %q to %q. The file with the same hash already exists!\n", sourcePath, targetPath)
						logger.Debugf("skip uploading a file %q to %q. The file with the same hash already exists!", sourcePath, targetPath)
						return nil
					}
				}
			}
		}
	} else {
		if !put.forceFlagValues.Force {
			// ask
			overwrite := commons.InputYN(fmt.Sprintf("file %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:                commons.TransferMethodPut,
					StartAt:               now,
					EndAt:                 now,
					SourcePath:            sourcePath,
					SourceSize:            sourceStat.Size(),
					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksum:          hex.EncodeToString(targetEntry.CheckSum),
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),
					Notes:                 []string{"no_overwrite", "skip"},
				}

				put.transferReportManager.AddFile(reportFile)

				commons.Printf("skip uploading a file %q to %q. The data object already exists!\n", sourcePath, targetPath)
				logger.Debugf("skip uploading a file %q to %q. The data object already exists!", sourcePath, targetPath)
				return nil
			}
		}
	}

	// schedule
	return put.schedulePut(sourceStat, sourcePath, tempPath, targetPath, requireEncryption, encryptionMode)
}

func (put *PutCommand) putDir(sourceStat fs.FileInfo, sourcePath string, targetPath string, parentEncryption bool, parentEncryptionMode commons.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "putDir",
	})

	commons.MarkIRODSPathMap(put.updatedPathMap, targetPath)

	if put.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			put.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a dir %q to %q. The dir is hidden!\n", sourcePath, targetPath)
			logger.Debugf("skip uploading a dir %q to %q. The dir is hidden!", sourcePath, targetPath)
			return nil
		}
	}

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			err = put.filesystem.MakeDir(targetPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make a collection %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			put.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			if put.syncFlagValues.Sync {
				// if it is sync, remove
				if put.forceFlagValues.Force {
					removeErr := put.filesystem.RemoveFile(targetPath, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "put"},
					}

					put.transferReportManager.AddFile(reportFile)

					if removeErr != nil {
						return removeErr
					}
				} else {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						removeErr := put.filesystem.RemoveFile(targetPath, true)

						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:     commons.TransferMethodDelete,
							StartAt:    now,
							EndAt:      now,
							SourcePath: targetPath,
							Error:      removeErr,
							Notes:      []string{"overwrite", "put"},
						}

						put.transferReportManager.AddFile(reportFile)

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

	requireEncryption, encryptionMode := put.requireEncryption(targetPath, parentEncryption, parentEncryptionMode)

	// get entries
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to list a directory %q: %w", sourcePath, err)
	}

	for _, entry := range entries {
		newEntryPath := commons.MakeTargetIRODSFilePath(put.filesystem, entry.Name(), targetPath)

		entryPath := filepath.Join(sourcePath, entry.Name())

		entryStat, err := os.Stat(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(entryPath)
			}

			return xerrors.Errorf("failed to stat %q: %w", entryPath, err)
		}

		if entryStat.IsDir() {
			// dir
			err = put.putDir(entryStat, entryPath, newEntryPath, requireEncryption, encryptionMode)
			if err != nil {
				return err
			}
		} else {
			// file
			if requireEncryption {
				// encrypt filename
				tempPath, newTargetPath, err := put.getPathsForEncryption(entryPath, targetPath)
				if err != nil {
					return xerrors.Errorf("failed to get encryption path for %q: %w", entryPath, err)
				}

				err = put.putFile(entryStat, entryPath, tempPath, newTargetPath, requireEncryption, encryptionMode)
				if err != nil {
					return err
				}
			} else {
				err = put.putFile(entryStat, entryPath, "", newEntryPath, requireEncryption, encryptionMode)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (put *PutCommand) deleteOnSuccess(sourcePath string) error {
	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceStat.IsDir() {
		return os.RemoveAll(sourcePath)
	}

	return os.Remove(sourcePath)
}

func (put *PutCommand) deleteExtra(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := put.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	return put.deleteExtraInternal(targetPath)
}

func (put *PutCommand) deleteExtraInternal(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "deleteExtraInternal",
	})

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		// file
		if _, ok := put.updatedPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra data object %q", targetPath)

			removeErr := put.filesystem.RemoveFile(targetPath, true)

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodDelete,
				StartAt:    now,
				EndAt:      now,
				SourcePath: targetPath,
				Error:      removeErr,
				Notes:      []string{"extra", "put"},
			}

			put.transferReportManager.AddFile(reportFile)

			if removeErr != nil {
				return removeErr
			}
		}

		return nil
	}

	// target is dir
	if _, ok := put.updatedPathMap[targetPath]; !ok {
		// extra dir
		logger.Debugf("removing an extra collection %q", targetPath)

		removeErr := put.filesystem.RemoveDir(targetPath, true, true)

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:     commons.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: targetPath,
			Error:      removeErr,
			Notes:      []string{"extra", "put", "dir"},
		}

		put.transferReportManager.AddFile(reportFile)

		if removeErr != nil {
			return removeErr
		}
	} else {
		// non extra dir
		// scan recursively
		entries, err := put.filesystem.List(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to list a collection %q: %w", targetPath, err)
		}

		for _, entry := range entries {
			newTargetPath := path.Join(targetPath, entry.Name)
			err = put.deleteExtraInternal(newTargetPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (put *PutCommand) getEncryptionManagerForEncryption(mode commons.EncryptionMode) *commons.EncryptionManager {
	manager := commons.NewEncryptionManager(mode)

	switch mode {
	case commons.EncryptionModeWinSCP, commons.EncryptionModePGP:
		manager.SetKey([]byte(put.encryptionFlagValues.Key))
	case commons.EncryptionModeSSH:
		manager.SetPublicPrivateKey(put.encryptionFlagValues.PublicPrivateKeyPath)
	}

	return manager
}

func (put *PutCommand) getPathsForEncryption(sourcePath string, targetPath string) (string, string, error) {
	if put.encryptionFlagValues.Mode != commons.EncryptionModeUnknown {
		encryptManager := put.getEncryptionManagerForEncryption(put.encryptionFlagValues.Mode)
		sourceFilename := commons.GetBasename(sourcePath)

		encryptedFilename, err := encryptManager.EncryptFilename(sourceFilename)
		if err != nil {
			return "", "", xerrors.Errorf("failed to encrypt filename %q: %w", sourcePath, err)
		}

		tempFilePath := commons.MakeTargetLocalFilePath(encryptedFilename, put.encryptionFlagValues.TempPath)

		targetFilePath := commons.MakeTargetIRODSFilePath(put.filesystem, encryptedFilename, targetPath)

		return tempFilePath, targetFilePath, nil
	}

	targetFilePath := commons.MakeTargetIRODSFilePath(put.filesystem, sourcePath, targetPath)

	return "", targetFilePath, nil
}

func (put *PutCommand) encryptFile(sourcePath string, encryptedFilePath string, encryptionMode commons.EncryptionMode) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PutCommand",
		"function": "encryptFile",
	})

	if encryptionMode != commons.EncryptionModeUnknown {
		logger.Debugf("encrypt a file %q to %q", sourcePath, encryptedFilePath)

		encryptManager := put.getEncryptionManagerForEncryption(encryptionMode)

		err := encryptManager.EncryptFile(sourcePath, encryptedFilePath)
		if err != nil {
			return false, xerrors.Errorf("failed to encrypt %q to %q: %w", sourcePath, encryptedFilePath, err)
		}

		return true, nil
	}

	return false, nil
}

func (put *PutCommand) calculateThreadForTransferJob(size int64) int {
	threads := commons.CalculateThreadForTransferJob(size, put.parallelTransferFlagValues.ThreadNumber)

	// determine how to upload
	if put.parallelTransferFlagValues.SingleThread || put.parallelTransferFlagValues.ThreadNumber == 1 {
		return 1
	} else if put.parallelTransferFlagValues.Icat && !put.filesystem.SupportParallelUpload() {
		return 1
	} else if put.parallelTransferFlagValues.RedirectToResource || put.parallelTransferFlagValues.Icat {
		return threads
	}

	//if size < commons.RedirectToResourceMinSize && !put.filesystem.SupportParallelUpload() {
	//	// icat
	//	return 1
	//}

	if !put.filesystem.SupportParallelUpload() {
		return 1
	}

	return threads
}

func (put *PutCommand) determineTransferMode(size int64) commons.TransferMode {
	threadsRequired := put.calculateThreadForTransferJob(size)

	if threadsRequired == 1 {
		return commons.TransferModeSingleThread
	}

	if put.parallelTransferFlagValues.SingleThread || put.parallelTransferFlagValues.ThreadNumber == 1 {
		return commons.TransferModeSingleThread
	} else if put.parallelTransferFlagValues.RedirectToResource {
		return commons.TransferModeRedirect
	} else if put.parallelTransferFlagValues.Icat {
		return commons.TransferModeICAT
	}

	// sysconfig
	systemConfig := commons.GetSystemConfig()
	if systemConfig != nil && systemConfig.AdditionalConfig != nil {
		if systemConfig.AdditionalConfig.TransferMode.Valid() {
			return systemConfig.AdditionalConfig.TransferMode
		}
	}

	// auto
	//if size >= commons.RedirectToResourceMinSize {
	//	return commons.TransferModeRedirect
	//}

	return commons.TransferModeICAT
}

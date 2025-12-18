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
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/encryption"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/parallel"
	commons_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/transfer"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	flag.SetChecksumFlags(putCmd)
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

	parallelTransferJobManager    *parallel.ParallelJobManager
	parallelPostProcessJobManager *parallel.ParallelJobManager
	transferReportManager         *transfer.TransferReportManager
	updatedPathMap                map[string]bool
	mutex                         sync.RWMutex // mutex for updatedPathMap
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
		return nil, errors.New("failed to put multiple source collections without creating root directory")
	}

	return put, nil
}

func (put *PutCommand) Process() error {
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(put.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrap(err, "failed to input missing fields")
	}

	// Create a file system
	put.account = config.GetSessionConfig().ToIRODSAccount()
	if len(put.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", put.ticketAccessFlagValues.Name)
		put.account.Ticket = put.ticketAccessFlagValues.Name
	}

	put.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(put.account, put.maxConnectionNum, put.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return errors.Wrap(err, "failed to get iRODS FS Client")
	}
	defer put.filesystem.Release()

	if put.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(put.filesystem, put.commonFlagValues.Timeout)
	}

	// transfer report
	put.transferReportManager, err = transfer.NewTransferReportManager(put.transferReportFlagValues.Report, put.transferReportFlagValues.ReportPath, put.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return errors.Wrap(err, "failed to create transfer report manager")
	}
	defer put.transferReportManager.Release()

	// set default key for encryption
	if len(put.encryptionFlagValues.Key) == 0 {
		put.encryptionFlagValues.Key = put.account.Password
	}

	// parallel job manager
	ioSession := put.filesystem.GetIOSession()
	put.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), put.progressFlagValues.ShowProgress, put.progressFlagValues.ShowFullPath)
	put.parallelPostProcessJobManager = parallel.NewParallelJobManager(1, put.progressFlagValues.ShowProgress, put.progressFlagValues.ShowFullPath)

	// run
	if len(put.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = put.ensureTargetIsDir(put.targetPath)
		if err != nil {
			return errors.Wrapf(err, "target path %q is not a directory", put.targetPath)
		}
	}

	for _, sourcePath := range put.sourcePaths {
		err = put.putOne(sourcePath, put.targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to put %q to %q", sourcePath, put.targetPath)
		}
	}

	// delete sources on success
	if put.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range put.sourcePaths {
			logger.Infof("deleting source file or directory under %q after upload", sourcePath)

			err = put.deleteOnSuccessOne(sourcePath)
			if err != nil {
				return errors.Wrapf(err, "failed to delete source %q after upload", sourcePath)
			}
		}
	}

	// delete extra
	if put.syncFlagValues.Delete {
		logger.Infof("deleting extra data objects and collections under %q", put.targetPath)

		err = put.deleteExtraOne(put.targetPath)
		if err != nil {
			return errors.Wrap(err, "failed to delete extra data objects or collections")
		}
	}

	logger.Info("done scheduling jobs, starting jobs")

	transferErr := put.parallelTransferJobManager.Start()
	if transferErr != nil {
		// error occurred while transferring files
		put.parallelPostProcessJobManager.CancelJobs()
	}

	postProcessErr := put.parallelPostProcessJobManager.Start()

	if transferErr != nil {
		return errors.Wrap(transferErr, "failed to perform transfer jobs")
	}

	if postProcessErr != nil {
		return errors.Wrap(postProcessErr, "failed to perform post process jobs")
	}

	return nil
}

func (put *PutCommand) ensureTargetIsDir(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := put.account.ClientZone
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// not exist
			return types.NewNotDirError(targetPath)
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if !targetEntry.IsDir() {
		return types.NewNotDirError(targetPath)
	}

	return nil
}

func (put *PutCommand) getEncryptionMode(targetPath string, parentEncryptionMode encryption.EncryptionMode) encryption.EncryptionMode {
	if put.encryptionFlagValues.Encryption {
		return put.encryptionFlagValues.Mode
	}

	if put.encryptionFlagValues.NoEncryption {
		return encryption.EncryptionModeNone
	}

	if !put.encryptionFlagValues.IgnoreMeta {
		// load encryption config from meta
		targetDir := targetPath

		targetEntry, err := put.filesystem.Stat(targetPath)
		if err != nil {
			if irodsclient_types.IsFileNotFoundError(err) {
				targetDir = path.Dir(targetPath)
			} else {
				return parentEncryptionMode
			}
		} else {
			if !targetEntry.IsDir() {
				targetDir = path.Dir(targetEntry.Path)
			}
		}

		encryptionConfig := encryption.GetEncryptionConfigFromMeta(put.filesystem, targetDir)
		return encryptionConfig.Mode
	}

	return parentEncryptionMode
}

func (put *PutCommand) putOne(sourcePath string, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := put.account.ClientZone
	sourcePath = commons_path.MakeLocalPath(sourcePath)
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceStat.IsDir() {
		// dir
		if !put.noRootFlagValues.NoRoot {
			targetPath = commons_path.MakeIRODSTargetFilePath(put.filesystem, sourcePath, targetPath)
		}

		return put.putDir(sourceStat, sourcePath, targetPath, encryption.EncryptionModeNone)
	}

	// file
	encryptionMode := put.getEncryptionMode(targetPath, encryption.EncryptionModeNone)
	if encryptionMode != encryption.EncryptionModeNone {
		// encrypt filename
		tempPath, newTargetPath, err := put.getPathsForEncryption(sourcePath, targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to get encryption path for %q", sourcePath)
		}

		return put.putFile(sourceStat, sourcePath, tempPath, newTargetPath, encryptionMode)
	}

	targetPath = commons_path.MakeIRODSTargetFilePath(put.filesystem, sourcePath, targetPath)
	return put.putFile(sourceStat, sourcePath, "", targetPath, encryption.EncryptionModeNone)
}

func (put *PutCommand) deleteOnSuccessOne(sourcePath string) error {
	sourcePath = commons_path.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceStat.IsDir() {
		// dir
		return put.deleteDirOnSuccess(sourcePath)
	}

	// file
	return put.deleteFileOnSuccess(sourcePath)
}

func (put *PutCommand) deleteExtraOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := put.account.ClientZone
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if targetEntry.IsDir() {
		// dir
		return put.deleteExtraDir(targetPath)
	}

	// file
	return put.deleteExtraFile(targetPath)
}

func (put *PutCommand) schedulePut(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, encryptionMode encryption.EncryptionMode) {
	logger := log.WithFields(log.Fields{
		"source_path":     sourcePath,
		"temp_path":       tempPath,
		"target_path":     targetPath,
		"encryption_mode": encryptionMode,
	})

	defaultNotes := []string{"put"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		put.transferReportManager.AddTransfer(result, transfer.TransferMethodPut, err, newNotes)
	}

	_, threadsRequired := put.determineTransferMethod(sourceStat.Size())

	putTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("upload", -1, sourceStat.Size(), true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for uploading a file")
			return nil
		}

		logger.Debug("uploading a file")

		notes := []string{}

		// encrypt
		if encryptionMode != encryption.EncryptionModeNone {
			notes = append(notes, "encrypt")

			job.Progress("encrypt", 0, sourceStat.Size(), false)

			_, encryptErr := put.encryptFile(sourcePath, tempPath, encryptionMode)
			if encryptErr != nil {
				job.Progress("encrypt", -1, sourceStat.Size(), true)

				reportSimple(encryptErr, notes...)
				return errors.Wrap(encryptErr, "failed to encrypt file")
			}

			defer func() {
				if len(tempPath) > 0 {
					// remove temp file
					logger.Debug("removing a temporary file")
					os.Remove(tempPath)
				}
			}()
		}

		progressCallbackPut := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		job.Progress("upload", 0, sourceStat.Size(), false)

		uploadSourcePath := sourcePath
		if len(tempPath) > 0 {
			uploadSourcePath = tempPath
		}

		parentTargetPath := path.Dir(targetPath)
		_, statErr := put.filesystem.Stat(parentTargetPath)
		if statErr != nil {
			// must exist, mkdir is performed at putDir
			job.Progress("upload", -1, sourceStat.Size(), true)

			reportSimple(statErr)
			return errors.Wrapf(statErr, "failed to stat %q", parentTargetPath)
		}

		uploadResult, uploadErr := put.filesystem.UploadFileParallel(uploadSourcePath, targetPath, "", threadsRequired, false, put.checksumFlagValues.VerifyChecksum, false, progressCallbackPut)
		notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))

		if uploadErr != nil {
			job.Progress("upload", -1, sourceStat.Size(), true)
			job.Progress("checksum", -1, sourceStat.Size(), true)

			reportTransfer(uploadResult, uploadErr, notes...)
			return errors.Wrapf(uploadErr, "failed to upload %q to %q", sourcePath, targetPath)
		}

		reportTransfer(uploadResult, nil, notes...)

		logger.Debug("uploaded a file")

		return nil
	}

	put.parallelTransferJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a file upload, %d threads", threadsRequired)
}

func (put *PutCommand) scheduleDeleteFileOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"put", "delete on success", "file"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodDelete,
			StartAt:    startTime,
			EndAt:      endTime,
			SourcePath: sourcePath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	deleteTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("delete", -1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for deleting a file")
			return nil
		}

		logger.Debug("deleting a file")

		job.Progress("delete", 0, 1, false)

		removeErr := os.Remove(sourcePath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(removeErr, "failed to delete %q", sourcePath)
		}

		logger.Debug("deleted a file")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	put.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled a file deletion")
}

func (put *PutCommand) scheduleDeleteDirOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"put", "delete on success", "directory"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodDelete,
			StartAt:    startTime,
			EndAt:      endTime,
			SourcePath: sourcePath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	deleteTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("delete", -1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for deleting an empty directory")
			return nil
		}

		logger.Debug("deleting an empty directory")

		job.Progress("delete", 0, 1, false)

		removeErr := os.Remove(sourcePath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(removeErr, "failed to delete %q", sourcePath)
		}

		logger.Debug("deleted an empty directory")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	put.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an empty directory deletion")
}

func (put *PutCommand) scheduleDeleteExtraFile(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"put", "extra", "file"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	deleteTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("delete", -1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for deleting extra data object")
			return nil
		}

		logger.Debug("deleting an extra data object")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := put.filesystem.RemoveFile(targetPath, true)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(removeErr, "failed to delete %q", targetPath)
		}

		logger.Debug("deleted an extra data object")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	put.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra data object deletion")
}

func (put *PutCommand) scheduleDeleteExtraDir(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"put", "extra", "directory"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	deleteTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("delete", -1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for deleting an extra collection")
			return nil
		}

		logger.Debug("deleting an extra collection")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := put.filesystem.RemoveDir(targetPath, false, false)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(removeErr, "failed to delete %q", targetPath)
		}

		logger.Debug("deleted an extra collection")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	put.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra collection deletion")
}

func (put *PutCommand) putFile(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, encryptionMode encryption.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"source_path":     sourcePath,
		"temp_path":       tempPath,
		"target_path":     targetPath,
		"encryption_mode": encryptionMode,
	})

	defaultNotes := []string{"put"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	put.mutex.Lock()
	commons_path.MarkIRODSPathMap(put.updatedPathMap, targetPath)
	put.mutex.Unlock()

	if put.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip uploading a file %q to %q. The file is hidden!\n", sourcePath, targetPath)
			logger.Debug("skip uploading a file. The file is hidden!")
			return nil
		}
	}

	if put.syncFlagValues.Age > 0 {
		// exclude old
		age := time.Since(sourceStat.ModTime())
		maxAge := time.Duration(put.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			reportSimple(nil, "age", "skipped")
			terminal.Printf("skip uploading a file %q to %q. The file is too old (%s > %s)!\n", sourcePath, targetPath, age, maxAge)
			logger.Debugf("skip uploading a file. The file is too old (%s > %s)!", age, maxAge)
			return nil
		}
	}

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			put.schedulePut(sourceStat, sourcePath, tempPath, targetPath, encryptionMode)
			return nil
		}

		reportSimple(err)
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if put.syncFlagValues.Sync {
			// if it is sync, remove
			if put.forceFlagValues.Force {
				startTime := time.Now()
				removeErr := put.filesystem.RemoveDir(targetPath, true, true)
				endTime := time.Now()
				reportOverwrite(startTime, endTime, removeErr, "directory")

				if removeErr != nil {
					return removeErr
				}

				// fallthrough to put
			} else {
				// ask
				overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a data object %q, but collection exists. Overwrite?", targetPath))
				if overwrite {
					startTime := time.Now()
					removeErr := put.filesystem.RemoveDir(targetPath, true, true)
					endTime := time.Now()
					reportOverwrite(startTime, endTime, removeErr, "directory")

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to put
				} else {
					overwriteErr := types.NewNotFileError(targetPath)

					now := time.Now()
					reportOverwrite(now, now, overwriteErr, "directory", "declined", "skipped")
					terminal.Printf("skip uploading a file %q to %q. Collection exists with the same name!\n", sourcePath, targetPath)
					logger.Debug("skip uploading a file. Collection exists with the same name!")
					return nil
				}
			}
		} else {
			notFileErr := types.NewNotFileError(targetPath)
			now := time.Now()
			reportOverwrite(now, now, notFileErr, "directory")
			return notFileErr
		}
	}

	if put.differentialTransferFlagValues.DifferentialTransfer {
		if put.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceStat.Size() {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                transfer.TransferMethodPut,
					StartAt:               now,
					EndAt:                 now,
					SourcePath:            sourcePath,
					SourceSize:            sourceStat.Size(),
					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),

					Notes: []string{"put", "file", "differential", "no_hash", "same size", "skipped"},
				}

				put.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip uploading a file %q to %q. The data object already exists!\n", sourcePath, targetPath)
				logger.Debug("skip uploading a file. The data object already exists!")
				return nil
			}
		} else {
			if targetEntry.Size == sourceStat.Size() {
				// compare hash
				if len(targetEntry.CheckSum) > 0 {
					localChecksum, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm), nil)
					if err != nil {
						reportSimple(err, "differential")
						return errors.Wrapf(err, "failed to get hash for %q", sourcePath)
					}

					if bytes.Equal(localChecksum, targetEntry.CheckSum) {
						// skip
						now := time.Now()
						reportFile := &transfer.TransferReportFile{
							Method:                  transfer.TransferMethodPut,
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

							Notes: []string{"put", "file", "differential", "same checksum", "skipped"},
						}

						put.transferReportManager.AddFile(reportFile)

						terminal.Printf("skip uploading a file %q to %q. The data object with the same hash already exists!\n", sourcePath, targetPath)
						logger.Debug("skip uploading a file. The data object with the same hash already exists!")
						return nil
					}
				}
			}
		}
	} else {
		if !put.forceFlagValues.Force {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Data object %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                transfer.TransferMethodPut,
					StartAt:               now,
					EndAt:                 now,
					SourcePath:            sourcePath,
					SourceSize:            sourceStat.Size(),
					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksum:          hex.EncodeToString(targetEntry.CheckSum),
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),

					Notes: []string{"put", "file", "overwrite", "declined", "skipped"},
				}

				put.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip uploading a file %q to %q. The data object already exists!\n", sourcePath, targetPath)
				logger.Debug("skip uploading a file. The data object already exists!")
				return nil
			}
		}
	}

	// schedule
	put.schedulePut(sourceStat, sourcePath, tempPath, targetPath, encryptionMode)
	return nil
}

func (put *PutCommand) putDir(sourceStat fs.FileInfo, sourcePath string, targetPath string, parentEncryptionMode encryption.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"source_path":            sourcePath,
		"target_path":            targetPath,
		"parent_encryption_mode": parentEncryptionMode,
	})

	defaultNotes := []string{"put", "directory"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    startTime,
			EndAt:      endTime,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	reportOverwrite := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	put.mutex.Lock()
	commons_path.MarkIRODSPathMap(put.updatedPathMap, targetPath)
	put.mutex.Unlock()

	if put.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip uploading a directory %q to %q. The directory is hidden!\n", sourcePath, targetPath)
			logger.Debug("skip uploading a directory. The directory is hidden!")
			return nil
		}
	}

	targetEntry, err := put.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			startTime := time.Now()
			err = put.filesystem.MakeDir(targetPath, true)
			endTime := time.Now()
			report(startTime, endTime, err)
			if err != nil {
				return errors.Wrapf(err, "failed to make a collection %q", targetPath)
			}

			// fallthrough to put entries
		} else {
			reportSimple(err)
			return errors.Wrapf(err, "failed to stat %q", targetPath)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			if put.syncFlagValues.Sync {
				// if it is sync, remove
				if put.forceFlagValues.Force {
					startTime := time.Now()
					removeErr := put.filesystem.RemoveFile(targetPath, true)
					endTime := time.Now()
					reportOverwrite(startTime, endTime, removeErr)

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to put entries
				} else {
					// ask
					overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						startTime := time.Now()
						removeErr := put.filesystem.RemoveFile(targetPath, true)
						endTime := time.Now()

						reportOverwrite(startTime, endTime, removeErr)

						if removeErr != nil {
							return removeErr
						}

						// fallthrough to put entries
					} else {
						overwriteErr := types.NewNotDirError(targetPath)

						now := time.Now()
						reportOverwrite(now, now, overwriteErr, "declined", "skipped")
						terminal.Printf("skip uploading a directory %q to %q. Data object exists with the same name!\n", sourcePath, targetPath)
						logger.Debug("skip uploading a directory. Data object exists with the same name!")
						return nil
					}
				}
			} else {
				notDirErr := types.NewNotDirError(targetPath)
				now := time.Now()
				reportOverwrite(now, now, notDirErr)
				return notDirErr
			}
		}
	}

	// load encryption config
	encryptionMode := put.getEncryptionMode(targetPath, parentEncryptionMode)
	logger.Infof("encryption mode for %q is %s", targetPath, encryptionMode)

	// get entries
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a directory %q", sourcePath)
	}

	for _, entry := range entries {
		newEntryPath := commons_path.MakeIRODSTargetFilePath(put.filesystem, entry.Name(), targetPath)

		entryPath := filepath.Join(sourcePath, entry.Name())
		entryStat, err := os.Stat(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				reportSimple(err)
				return irodsclient_types.NewFileNotFoundError(entryPath)
			}

			return errors.Wrapf(err, "failed to stat %q", entryPath)
		}

		if entryStat.IsDir() {
			// dir
			err = put.putDir(entryStat, entryPath, newEntryPath, encryptionMode)
			if err != nil {
				return err
			}
		} else {
			// file
			if encryptionMode != encryption.EncryptionModeNone {
				// encrypt filename
				tempPath, newTargetPath, err := put.getPathsForEncryption(entryPath, targetPath)
				if err != nil {
					reportSimple(err)
					return errors.Wrapf(err, "failed to get encryption path for %q", entryPath)
				}

				err = put.putFile(entryStat, entryPath, tempPath, newTargetPath, encryptionMode)
				if err != nil {
					return err
				}
			} else {
				err = put.putFile(entryStat, entryPath, "", newEntryPath, encryptionMode)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (put *PutCommand) deleteFileOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"put", "delete on success", "file"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a file after upload", sourcePath)

	if put.forceFlagValues.Force {
		put.scheduleDeleteFileOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a file %q after upload. Remove?", sourcePath))
		if overwrite {
			put.scheduleDeleteFileOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (put *PutCommand) deleteDirOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"put", "delete on success", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			Error:      err,
			Notes:      newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a directory after upload", sourcePath)

	// scan recursively
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a directory %q", sourcePath)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// dir
			newSourcePath := filepath.Join(sourcePath, entry.Name())
			err = put.deleteDirOnSuccess(newSourcePath)
			if err != nil {
				return err
			}
		} else {
			// file
			newSourcePath := filepath.Join(sourcePath, entry.Name())
			err = put.deleteFileOnSuccess(newSourcePath)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	if put.forceFlagValues.Force {
		put.scheduleDeleteDirOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a directory %q after upload. Remove?", sourcePath))
		if overwrite {
			put.scheduleDeleteDirOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (put *PutCommand) deleteExtraFile(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"put", "extra", "file"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	put.mutex.RLock()
	isExtra := false
	if _, ok := put.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	put.mutex.RUnlock()

	if isExtra {
		// extra file
		logger.Debug("removing an extra data object", targetPath)

		if put.forceFlagValues.Force {
			put.scheduleDeleteExtraFile(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra data object %q. Remove?", targetPath))
			if overwrite {
				put.scheduleDeleteExtraFile(targetPath)
				return nil
			} else {
				// do not remove
				reportSimple(nil, "declined", "skipped")
				return nil
			}
		}
	}

	return nil
}

func (put *PutCommand) deleteExtraDir(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"put", "extra", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		put.transferReportManager.AddFile(reportFile)
	}

	// scan recursively
	entries, err := put.filesystem.List(targetPath)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a collection %q", targetPath)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// dir
			err = put.deleteExtraDir(entry.Path)
			if err != nil {
				return err
			}
		} else {
			// file
			err = put.deleteExtraFile(entry.Path)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	put.mutex.RLock()
	isExtra := false
	if _, ok := put.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	put.mutex.RUnlock()

	if isExtra {
		// extra dir
		logger.Debug("removing an extra collection")

		if put.forceFlagValues.Force {
			put.scheduleDeleteExtraDir(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra collection %q. Remove?", targetPath))
			if overwrite {
				put.scheduleDeleteExtraDir(targetPath)
				return nil
			} else {
				// do not remove
				reportSimple(nil, "declined", "skipped")
				return nil
			}
		}
	}

	return nil
}

func (put *PutCommand) getEncryptionManagerForEncryption(mode encryption.EncryptionMode) *encryption.EncryptionManager {
	manager := encryption.NewEncryptionManager(mode)

	switch mode {
	case encryption.EncryptionModeWinSCP, encryption.EncryptionModePGP:
		manager.SetKey([]byte(put.encryptionFlagValues.Key))
	case encryption.EncryptionModeSSH:
		manager.SetPublicPrivateKey(put.encryptionFlagValues.PublicPrivateKeyPath)
	}

	return manager
}

func (put *PutCommand) getPathsForEncryption(sourcePath string, targetPath string) (string, string, error) {
	if put.encryptionFlagValues.Mode != encryption.EncryptionModeNone {
		encryptManager := put.getEncryptionManagerForEncryption(put.encryptionFlagValues.Mode)
		sourceFilename := filepath.Base(sourcePath)

		encryptedFilename, err := encryptManager.EncryptFilename(sourceFilename)
		if err != nil {
			return "", "", errors.Wrapf(err, "failed to encrypt filename %q", sourcePath)
		}

		tempFilePath := commons_path.MakeLocalTargetFilePath(encryptedFilename, put.encryptionFlagValues.TempPath)

		targetFilePath := commons_path.MakeIRODSTargetFilePath(put.filesystem, encryptedFilename, targetPath)

		return tempFilePath, targetFilePath, nil
	}

	targetFilePath := commons_path.MakeIRODSTargetFilePath(put.filesystem, sourcePath, targetPath)

	return "", targetFilePath, nil
}

func (put *PutCommand) encryptFile(sourcePath string, encryptedFilePath string, encryptionMode encryption.EncryptionMode) (bool, error) {
	logger := log.WithFields(log.Fields{
		"source_path":     sourcePath,
		"temp_path":       encryptedFilePath,
		"encryption_mode": encryptionMode,
	})

	if encryptionMode != encryption.EncryptionModeNone {
		logger.Debug("encrypt a file")

		encryptManager := put.getEncryptionManagerForEncryption(encryptionMode)

		err := encryptManager.EncryptFile(sourcePath, encryptedFilePath)
		if err != nil {
			return false, errors.Wrapf(err, "failed to encrypt %q to %q", sourcePath, encryptedFilePath)
		}

		return true, nil
	}

	return false, nil
}

func (put *PutCommand) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	threads := parallel.CalculateThreadForTransferJob(size, put.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to upload
	if put.parallelTransferFlagValues.SingleThread || put.parallelTransferFlagValues.ThreadNumber <= 2 || put.parallelTransferFlagValues.ThreadNumberPerFile == 1 || !put.filesystem.SupportParallelUpload() {
		threads = 1
	}

	if put.parallelTransferFlagValues.Icat {
		return transfer.TransferModeICAT, threads
	}

	// sysconfig
	systemConfig := config.GetSystemConfig()
	if systemConfig != nil && systemConfig.AdditionalConfig != nil {
		mode := transfer.GetTransferMode(systemConfig.AdditionalConfig.TransferMode)
		if mode.Valid() {
			return mode, threads
		}
	}

	return transfer.TransferModeICAT, threads
}

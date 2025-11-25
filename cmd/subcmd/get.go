package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
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
	"github.com/cyverse/gocommands/commons/wildcard"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var getCmd = &cobra.Command{
	Use:     "get <data-object-or-collection>... <dest-local-file-or-dir>",
	Aliases: []string{"iget", "download"},
	Short:   "Download iRODS data objects or collections to a local file or directory",
	Long:    `This command downloads iRODS data objects or collections to the specified local file or directory.`,
	RunE:    processGetCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(getCmd, false)

	flag.SetBundleTransferFlags(getCmd, true, true)
	flag.SetParallelTransferFlags(getCmd, false, false)
	flag.SetForceFlags(getCmd, false)
	flag.SetRecursiveFlags(getCmd, true)
	flag.SetTicketAccessFlags(getCmd)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetDifferentialTransferFlags(getCmd, false)
	flag.SetChecksumFlags(getCmd)
	flag.SetNoRootFlags(getCmd)
	flag.SetSyncFlags(getCmd, true)
	flag.SetDecryptionFlags(getCmd)
	flag.SetPostTransferFlagValues(getCmd)
	flag.SetHiddenFileFlags(getCmd)
	flag.SetTransferReportFlags(getCmd)
	flag.SetWildcardSearchFlags(getCmd)

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
	decryptionFlagValues           *flag.DecryptionFlagValues
	postTransferFlagValues         *flag.PostTransferFlagValues
	hiddenFileFlagValues           *flag.HiddenFileFlagValues
	transferReportFlagValues       *flag.TransferReportFlagValues
	wildcardSearchFlagValues       *flag.WildcardSearchFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string

	parallelTransferJobManager    *parallel.ParallelJobManager
	parallelPostProcessJobManager *parallel.ParallelJobManager

	transferReportManager *transfer.TransferReportManager
	updatedPathMap        map[string]bool
	mutex                 sync.RWMutex // mutex for updatedPathMap
}

func NewGetCommand(command *cobra.Command, args []string) (*GetCommand, error) {
	get := &GetCommand{
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
		decryptionFlagValues:           flag.GetDecryptionFlagValues(command),
		postTransferFlagValues:         flag.GetPostTransferFlagValues(),
		hiddenFileFlagValues:           flag.GetHiddenFileFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),
		wildcardSearchFlagValues:       flag.GetWildcardSearchFlagValues(),

		updatedPathMap: map[string]bool{},
	}

	get.maxConnectionNum = get.parallelTransferFlagValues.ThreadNumber

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
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(get.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	get.account = config.GetSessionConfig().ToIRODSAccount()
	if len(get.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", get.ticketAccessFlagValues.Name)
		get.account.Ticket = get.ticketAccessFlagValues.Name
	}

	get.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(get.account, get.maxConnectionNum, get.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer get.filesystem.Release()

	if get.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(get.filesystem, get.commonFlagValues.Timeout)
	}

	// transfer report
	get.transferReportManager, err = transfer.NewTransferReportManager(get.transferReportFlagValues.Report, get.transferReportFlagValues.ReportPath, get.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer get.transferReportManager.Release()

	// set default key for decryption
	if len(get.decryptionFlagValues.Key) == 0 {
		get.decryptionFlagValues.Key = get.account.Password
	}

	// parallel job manager
	ioSession := get.filesystem.GetIOSession()
	get.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), get.progressFlagValues.ShowProgress, get.progressFlagValues.ShowFullPath)
	get.parallelPostProcessJobManager = parallel.NewParallelJobManager(1, get.progressFlagValues.ShowProgress, get.progressFlagValues.ShowFullPath)

	// Expand wildcards
	if get.wildcardSearchFlagValues.WildcardSearch {
		get.sourcePaths, err = wildcard.ExpandWildcards(get.filesystem, get.account, get.sourcePaths, true, true)
		if err != nil {
			return xerrors.Errorf("failed to expand wildcards:  %w", err)
		}
	}

	// run
	if len(get.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = get.ensureTargetIsDir(get.targetPath)
		if err != nil {
			return xerrors.Errorf("target path %q is not a directory: %w", get.targetPath, err)
		}
	}

	for _, sourcePath := range get.sourcePaths {
		err = get.getOne(sourcePath, get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get %q to %q: %w", sourcePath, get.targetPath, err)
		}
	}

	// delete sources on success
	if get.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range get.sourcePaths {
			logger.Infof("deleting source data objects and collections under %q after download", sourcePath)

			err = get.deleteOnSuccessOne(sourcePath)
			if err != nil {
				return xerrors.Errorf("failed to delete %q after download: %w", sourcePath, err)
			}
		}
	}

	// delete extra
	if get.syncFlagValues.Delete {
		logger.Infof("deleting extra files and directories under %q", get.targetPath)

		err := get.deleteExtraOne(get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files or directories: %w", err)
		}
	}

	logger.Info("done scheduling jobs, starting jobs")

	transferErr := get.parallelTransferJobManager.Start()
	if transferErr != nil {
		// error occurred while transferring files
		get.parallelPostProcessJobManager.CancelJobs()
	}

	postProcessErr := get.parallelPostProcessJobManager.Start()

	if transferErr != nil {
		return xerrors.Errorf("failed to perform transfer jobs: %w", transferErr)
	}

	if postProcessErr != nil {
		return xerrors.Errorf("failed to perform post process jobs: %w", err)
	}

	return nil
}

func (get *GetCommand) ensureTargetIsDir(targetPath string) error {
	targetPath = commons_path.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// not exist
			return types.NewNotDirError(targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetStat.IsDir() {
		return types.NewNotDirError(targetPath)
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

	mode := encryption.DetectEncryptionMode(sourcePath)
	return mode != encryption.EncryptionModeNone
}

func (get *GetCommand) hasTransferStatusFile(targetPath string) bool {
	// check transfer status file
	trxStatusFilePath := irodsclient_irodsfs.GetDataObjectTransferStatusFilePath(targetPath)
	_, err := os.Stat(trxStatusFilePath)
	return err == nil
}

func (get *GetCommand) getOne(sourcePath string, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := get.account.ClientZone
	sourcePath = commons_path.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons_path.MakeLocalPath(targetPath)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		// dir
		if !get.noRootFlagValues.NoRoot {
			targetPath = commons_path.MakeLocalTargetFilePath(sourcePath, targetPath)
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

	targetPath = commons_path.MakeLocalTargetFilePath(sourcePath, targetPath)
	return get.getFile(sourceEntry, "", targetPath)
}

func (get *GetCommand) deleteOnSuccessOne(sourcePath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := get.account.ClientZone
	sourcePath = commons_path.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		// dir
		return get.deleteDirOnSuccess(sourcePath)
	}

	// file
	return get.deleteFileOnSuccess(sourcePath)
}

func (get *GetCommand) deleteExtraOne(targetPath string) error {
	targetPath = commons_path.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if targetStat.IsDir() {
		// dir
		return get.deleteExtraDir(targetPath)
	}

	// file
	return get.deleteExtraFile(targetPath)
}

func (get *GetCommand) scheduleGet(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"temp_path":   tempPath,
		"target_path": targetPath,
	})

	defaultNotes := []string{"get"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		get.transferReportManager.AddTransfer(result, transfer.TransferMethodGet, err, newNotes)
	}

	_, threadsRequired := get.determineTransferMethod(sourceEntry.Size)

	getTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("download", -1, sourceEntry.Size, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for downloading a data object")
			return nil
		}

		logger.Debug("downloading a data object")

		progressCallbackGet := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		job.Progress("download", 0, sourceEntry.Size, false)

		downloadPath := targetPath
		if len(tempPath) > 0 {
			downloadPath = tempPath
		}

		parentDownloadPath := filepath.Dir(downloadPath)
		_, statErr := os.Stat(parentDownloadPath)
		if statErr != nil {
			// must exist, mkdir is performed at getDir
			job.Progress("download", -1, sourceEntry.Size, true)

			reportSimple(statErr)
			return xerrors.Errorf("failed to stat %q: %w", parentDownloadPath, statErr)
		}

		notes := []string{"icat", fmt.Sprintf("%d threads", threadsRequired)}
		if get.requireDecryption(sourceEntry.Path) {
			notes = append(notes, "decrypt")
		}

		downloadResult, downloadErr := get.filesystem.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, threadsRequired, get.checksumFlagValues.VerifyChecksum, progressCallbackGet)
		if downloadErr != nil {
			job.Progress("download", -1, sourceEntry.Size, true)
			job.Progress("checksum", -1, sourceEntry.Size, true)

			reportTransfer(downloadResult, downloadErr, notes...)
			return xerrors.Errorf("failed to download %q to %q: %w", sourceEntry.Path, targetPath, downloadErr)
		}

		// decrypt
		if get.requireDecryption(sourceEntry.Path) {
			job.Progress("decrypt", 0, sourceEntry.Size, false)

			_, decryptErr := get.decryptFile(sourceEntry.Path, tempPath, targetPath)
			if decryptErr != nil {
				job.Progress("decrypt", -1, sourceEntry.Size, true)

				reportTransfer(downloadResult, decryptErr, notes...)
				return xerrors.Errorf("failed to decrypt file: %w", decryptErr)
			}

			job.Progress("decrypt", sourceEntry.Size, sourceEntry.Size, false)
		}

		reportTransfer(downloadResult, nil, notes...)

		logger.Debug("downloaded a data object")

		return nil
	}

	get.parallelTransferJobManager.Schedule(sourceEntry.Path, getTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a data object download, %d threads", threadsRequired)
}

func (get *GetCommand) scheduleDeleteFileOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"get", "delete on success", "file"}

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

		get.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting a data object")
			return nil
		}

		logger.Debug("deleting a data object")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := get.filesystem.RemoveFile(sourcePath, true)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", sourcePath, removeErr)
		}

		logger.Debug("deleted a data object")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	get.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled a data object deletion")
}

func (get *GetCommand) scheduleDeleteDirOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"get", "delete on success", "directory"}

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

		get.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting an empty collection")
			return nil
		}

		logger.Debug("deleting an empty collection")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := get.filesystem.RemoveDir(sourcePath, false, false)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", sourcePath, removeErr)
		}

		logger.Debug("deleted an empty collection")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	get.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an empty collection deletion")
}

func (get *GetCommand) scheduleDeleteExtraFile(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"get", "extra", "file"}

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

		get.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting an extra file")
			return nil
		}

		logger.Debug("deleting an extra file")

		job.Progress("delete", 0, 1, false)

		removeErr := os.Remove(targetPath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", targetPath, removeErr)
		}

		logger.Debug("deleted an extra file")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	get.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra file deletion")
}

func (get *GetCommand) scheduleDeleteExtraDir(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"get", "extra", "directory"}

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

		get.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting an extra directory")
			return nil
		}

		logger.Debug("deleting an extra directory")

		job.Progress("delete", 0, 1, false)

		removeErr := os.RemoveAll(targetPath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", targetPath, removeErr)
		}

		logger.Debug("deleted an extra directory")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	get.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra directory deletion")
}

func (get *GetCommand) getFile(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"temp_path":   tempPath,
		"target_path": targetPath,
	})

	defaultNotes := []string{"get"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	get.mutex.Lock()
	commons_path.MarkLocalPathMap(get.updatedPathMap, targetPath)
	get.mutex.Unlock()

	if get.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip downloading a data object %q to %q. The data object is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debug("skip downloading a data object. The data object is hidden!")
			return nil
		}
	}

	if get.syncFlagValues.Age > 0 {
		// exclude old
		age := time.Since(sourceEntry.ModifyTime)
		maxAge := time.Duration(get.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			reportSimple(nil, "age", "skipped")
			terminal.Printf("skip downloading a data object %q to %q. The data object is too old (%s > %s)!\n", sourceEntry.Path, targetPath, age, maxAge)
			logger.Debugf("skip downloading a data object. The data object is too old (%s > %s)!", age, maxAge)
			return nil
		}
	}

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a file with new name
			get.scheduleGet(sourceEntry, tempPath, targetPath)
			return nil
		}

		reportSimple(err)
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetStat.IsDir() {
		if get.syncFlagValues.Sync {
			// if it is sync, remove
			if get.forceFlagValues.Force {
				removeErr := os.RemoveAll(targetPath)
				reportOverwrite(removeErr, "directory")

				if removeErr != nil {
					return removeErr
				}

				// fallthrough to get
			} else {
				// ask
				overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a file %q, but directory exists. Overwrite?", targetPath))
				if overwrite {
					removeErr := os.RemoveAll(targetPath)
					reportOverwrite(removeErr, "directory")

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to get
				} else {
					overwriteErr := types.NewNotFileError(targetPath)

					reportOverwrite(overwriteErr, "directory", "declined", "skipped")
					terminal.Printf("skip downloading a data object %q to %q. Directory exists with the same name!\n", sourceEntry.Path, targetPath)
					logger.Debug("skip downloading a data object. Directory exists with the same name!")
					return nil
				}
			}
		} else {
			notFileErr := types.NewNotFileError(targetPath)
			reportOverwrite(notFileErr, "directory")
			return notFileErr
		}
	}

	// check transfer status file
	if get.hasTransferStatusFile(targetPath) {
		// incomplete file - resume downloading
		terminal.Printf("resume downloading a data object %q\n", targetPath)
		logger.Debug("resume downloading a data object")

		get.scheduleGet(sourceEntry, tempPath, targetPath)
		return nil
	}

	if get.differentialTransferFlagValues.DifferentialTransfer {
		if get.differentialTransferFlagValues.NoHash {
			if targetStat.Size() == sourceEntry.Size {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                  transfer.TransferMethodGet,
					StartAt:                 now,
					EndAt:                   now,
					SourcePath:              sourceEntry.Path,
					SourceSize:              sourceEntry.Size,
					SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
					DestPath:                targetPath,
					DestSize:                targetStat.Size(),

					Notes: []string{"get", "file", "differential", "no_hash", "same size", "skipped"},
				}

				get.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip downloading a data object %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debug("skip downloading a data object. The file already exists!")
				return nil
			}
		} else {
			if targetStat.Size() == sourceEntry.Size {
				// compare hash
				if len(sourceEntry.CheckSum) > 0 {
					localChecksum, err := irodsclient_util.HashLocalFile(targetPath, string(sourceEntry.CheckSumAlgorithm), nil)
					if err != nil {
						reportSimple(err, "differential")
						return xerrors.Errorf("failed to get hash of %q: %w", targetPath, err)
					}

					if bytes.Equal(sourceEntry.CheckSum, localChecksum) {
						// skip
						now := time.Now()
						reportFile := &transfer.TransferReportFile{
							Method:                  transfer.TransferMethodGet,
							StartAt:                 now,
							EndAt:                   now,
							SourcePath:              sourceEntry.Path,
							SourceSize:              sourceEntry.Size,
							SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
							SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
							DestPath:                targetPath,
							DestSize:                targetStat.Size(),
							DestChecksum:            hex.EncodeToString(localChecksum),
							DestChecksumAlgorithm:   string(sourceEntry.CheckSumAlgorithm),

							Notes: []string{"get", "file", "differential", "same checksum", "skipped"},
						}

						get.transferReportManager.AddFile(reportFile)

						terminal.Printf("skip downloading a data object %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
						logger.Debug("skip downloading a data object. The file with the same hash already exists!")
						return nil
					}
				}
			}
		}
	} else {
		if !get.forceFlagValues.Force {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("File %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                  transfer.TransferMethodGet,
					StartAt:                 now,
					EndAt:                   now,
					SourcePath:              sourceEntry.Path,
					SourceSize:              sourceEntry.Size,
					SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
					DestPath:                targetPath,
					DestSize:                targetStat.Size(),

					Notes: []string{"get", "file", "overwrite", "decliened", "skipped"},
				}

				get.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip downloading a data object %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debug("skip downloading a data object. The file already exists!")
				return nil
			}
		}
	}

	// schedule
	get.scheduleGet(sourceEntry, tempPath, targetPath)
	return nil
}

func (get *GetCommand) getDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	defaultNotes := []string{"get", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	get.mutex.Lock()
	commons_path.MarkLocalPathMap(get.updatedPathMap, targetPath)
	get.mutex.Unlock()

	if get.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip downloading a collection %q to %q. The collection is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debug("skip downloading a collection. The collection is hidden!")
			return nil
		}
	}

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a directorywith new name
			err = os.MkdirAll(targetPath, 0766)
			reportSimple(err)
			if err != nil {
				return xerrors.Errorf("failed to make a directory %q: %w", targetPath, err)
			}

			// fallthrough to get entries
		} else {
			reportSimple(err)
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetStat.IsDir() {
			if get.syncFlagValues.Sync {
				// if it is sync, remove
				if get.forceFlagValues.Force {
					removeErr := os.Remove(targetPath)
					reportOverwrite(removeErr)

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to get entries
				} else {
					// ask
					overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						removeErr := os.Remove(targetPath)
						reportOverwrite(removeErr)

						if removeErr != nil {
							return removeErr
						}

						// fallthrough to get entries
					} else {
						overwriteErr := types.NewNotDirError(targetPath)

						reportOverwrite(overwriteErr, "declined", "skipped")
						terminal.Printf("skip downloading a collection %q to %q. File exists with the same name!\n", sourceEntry.Path, targetPath)
						logger.Debug("skip downloading a collection. File exists with the same name!")
						return nil
					}
				}
			} else {
				notDirErr := types.NewNotDirError(targetPath)
				reportOverwrite(notDirErr)
				return notDirErr
			}
		}
	}

	// load encryption config
	requireDecryption := get.requireDecryption(sourceEntry.Path)

	// get entries
	entries, err := get.filesystem.List(sourceEntry.Path)
	if err != nil {
		reportSimple(err)
		return xerrors.Errorf("failed to list a directory %q: %w", sourceEntry.Path, err)
	}

	for _, entry := range entries {
		newEntryPath := commons_path.MakeLocalTargetFilePath(entry.Path, targetPath)

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
					reportSimple(err)
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

func (get *GetCommand) deleteFileOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"get", "delete on success", "file"}

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

		get.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a data object after download")

	if get.forceFlagValues.Force {
		get.scheduleDeleteFileOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a data object %q after download. Remove?", sourcePath))
		if overwrite {
			get.scheduleDeleteFileOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (get *GetCommand) deleteDirOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"get", "delete on success", "directory"}

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

		get.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a collection after download")

	// scan recursively
	entries, err := get.filesystem.List(sourcePath)
	if err != nil {
		reportSimple(err)
		return xerrors.Errorf("failed to list a collection %q: %w", sourcePath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// dir
			err = get.deleteDirOnSuccess(entry.Path)
			if err != nil {
				return err
			}
		} else {
			// file
			err = get.deleteFileOnSuccess(entry.Path)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	if get.forceFlagValues.Force {
		get.scheduleDeleteDirOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a collection after download %q. Remove?", sourcePath))
		if overwrite {
			get.scheduleDeleteDirOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (get *GetCommand) deleteExtraFile(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"get", "extra", "file"}

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

		get.transferReportManager.AddFile(reportFile)
	}

	get.mutex.RLock()
	isExtra := false
	if _, ok := get.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	get.mutex.RUnlock()

	if isExtra {
		// extra file
		logger.Debug("removing an extra file")

		if get.forceFlagValues.Force {
			get.scheduleDeleteExtraFile(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra file %q. Remove?", targetPath))
			if overwrite {
				get.scheduleDeleteExtraFile(targetPath)
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

func (get *GetCommand) deleteExtraDir(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"get", "extra", "directory"}

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

		get.transferReportManager.AddFile(reportFile)
	}

	// scan recursively
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		reportSimple(err)
		return xerrors.Errorf("failed to list a directory %q: %w", targetPath, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(targetPath, entry.Name())

		if entry.IsDir() {
			// dir
			err = get.deleteExtraDir(entryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = get.deleteExtraFile(entryPath)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	get.mutex.RLock()
	isExtra := false
	if _, ok := get.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	get.mutex.RUnlock()

	if isExtra {
		// extra dir
		logger.Debug("removing an extra directory")

		if get.forceFlagValues.Force {
			get.scheduleDeleteExtraDir(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra directory %q. Remove?", targetPath))
			if overwrite {
				get.scheduleDeleteExtraDir(targetPath)
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

func (get *GetCommand) getEncryptionManagerForDecryption(mode encryption.EncryptionMode) *encryption.EncryptionManager {
	manager := encryption.NewEncryptionManager(mode)

	switch mode {
	case encryption.EncryptionModeWinSCP, encryption.EncryptionModePGP:
		manager.SetKey([]byte(get.decryptionFlagValues.Key))
	case encryption.EncryptionModeSSH:
		manager.SetPublicPrivateKey(get.decryptionFlagValues.PrivateKeyPath)
	}

	return manager
}

func (get *GetCommand) getPathsForDecryption(sourcePath string, targetPath string) (string, string, error) {
	encryptionMode := encryption.DetectEncryptionMode(sourcePath)

	if encryptionMode != encryption.EncryptionModeNone {
		// encrypted file
		sourceFilename := path.Base(sourcePath)
		encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

		tempFilePath := commons_path.MakeLocalTargetFilePath(sourcePath, get.decryptionFlagValues.TempPath)

		decryptedFilename, err := encryptManager.DecryptFilename(sourceFilename)
		if err != nil {
			return "", "", xerrors.Errorf("failed to decrypt filename %q: %w", sourcePath, err)
		}

		targetFilePath := commons_path.MakeLocalTargetFilePath(decryptedFilename, targetPath)

		return tempFilePath, targetFilePath, nil
	}

	targetFilePath := commons_path.MakeLocalTargetFilePath(sourcePath, targetPath)

	return "", targetFilePath, nil
}

func (get *GetCommand) decryptFile(sourcePath string, encryptedFilePath string, targetPath string) (bool, error) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
		"temp_path":   encryptedFilePath,
		"target_path": targetPath,
	})

	encryptionMode := encryption.DetectEncryptionMode(sourcePath)

	if encryptionMode != encryption.EncryptionModeNone {
		logger.Debug("decrypt a data object")

		encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

		err := encryptManager.DecryptFile(encryptedFilePath, targetPath)
		if err != nil {
			return false, xerrors.Errorf("failed to decrypt %q to %q: %w", encryptedFilePath, targetPath, err)
		}

		logger.Debug("removing a temp file")
		os.Remove(encryptedFilePath)

		return true, nil
	}

	return false, nil
}

func (get *GetCommand) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	threads := parallel.CalculateThreadForTransferJob(size, get.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to download
	if get.parallelTransferFlagValues.SingleThread || get.parallelTransferFlagValues.ThreadNumber <= 2 || get.parallelTransferFlagValues.ThreadNumberPerFile == 1 {
		threads = 1
	}

	if get.parallelTransferFlagValues.Icat {
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

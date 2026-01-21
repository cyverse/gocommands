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
	"github.com/cyverse/gocommands/commons/bundle"
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

var bputCmd = &cobra.Command{
	Use:     "bput <local-file-or-dir>... <dest-collection>",
	Aliases: []string{"bundle_put", "bundle_upload"},
	Short:   "Bundle-upload files or directories to an iRODS collection",
	Long:    `This command uploads files or directories to the specified iRODS collection. The files or directories are first bundled with TAR to optimize data transfer bandwidth and then extracted in iRODS after upload.`,
	RunE:    processBputCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bputCmd, false)

	flag.SetBundleTransferFlags(bputCmd, false, false)
	flag.SetParallelTransferFlags(bputCmd, false, false)
	flag.SetForceFlags(bputCmd, true)
	flag.SetRecursiveFlags(bputCmd, true)
	flag.SetProgressFlags(bputCmd)
	flag.SetRetryFlags(bputCmd)
	flag.SetDifferentialTransferFlags(bputCmd, false)
	flag.SetChecksumFlags(bputCmd)
	flag.SetNoRootFlags(bputCmd)
	flag.SetSyncFlags(bputCmd, true)
	flag.SetEncryptionFlags(bputCmd)
	flag.SetHiddenFileFlags(bputCmd)
	flag.SetPostTransferFlagValues(bputCmd)
	flag.SetTransferReportFlags(bputCmd)

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	bput, err := NewBputCommand(command, args)
	if err != nil {
		return err
	}

	return bput.Process()
}

type BputCommand struct {
	command *cobra.Command

	commonFlagValues               *flag.CommonFlagValues
	bundleTransferFlagValues       *flag.BundleTransferFlagValues
	parallelTransferFlagValues     *flag.ParallelTransferFlagValues
	forceFlagValues                *flag.ForceFlagValues
	recursiveFlagValues            *flag.RecursiveFlagValues
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

	stagingPath string

	parallelTransferJobManager    *parallel.ParallelJobManager
	parallelPostProcessJobManager *parallel.ParallelJobManager
	bundleManager                 *bundle.BundleManager
	transferReportManager         *transfer.TransferReportManager
	updatedPathMap                map[string]bool
	mutex                         sync.RWMutex // mutex for updatedPathMap

	totalUploadedFiles int
	totalUploadedBytes int64
	startTime          time.Time
}

func NewBputCommand(command *cobra.Command, args []string) (*BputCommand, error) {
	bput := &BputCommand{
		command: command,

		commonFlagValues:               flag.GetCommonFlagValues(command),
		bundleTransferFlagValues:       flag.GetBundleTransferFlagValues(),
		parallelTransferFlagValues:     flag.GetParallelTransferFlagValues(),
		forceFlagValues:                flag.GetForceFlagValues(),
		recursiveFlagValues:            flag.GetRecursiveFlagValues(),
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

		updatedPathMap:     map[string]bool{},
		totalUploadedFiles: 0,
		totalUploadedBytes: 0,
		startTime:          time.Now(),
	}

	bput.maxConnectionNum = bput.parallelTransferFlagValues.ThreadNumber

	// path
	bput.targetPath = "./"
	bput.sourcePaths = args

	if len(args) >= 2 {
		bput.targetPath = args[len(args)-1]
		bput.sourcePaths = args[:len(args)-1]
	}

	if bput.noRootFlagValues.NoRoot && len(bput.sourcePaths) > 1 {
		return nil, errors.New("failed to put multiple source collections without creating root directory")
	}

	return bput, nil
}

func (bput *BputCommand) Process() error {
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(bput.command)
	if err != nil {
		return errors.Wrap(err, "failed to process common flags")
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
	bput.account = config.GetSessionConfig().ToIRODSAccount()
	bput.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(bput.account, bput.maxConnectionNum, bput.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return errors.Wrap(err, "failed to get iRODS FS Client")
	}
	defer bput.filesystem.Release()

	if bput.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(bput.filesystem, bput.commonFlagValues.Timeout)
	}

	// transfer report
	bput.transferReportManager, err = transfer.NewTransferReportManager(bput.transferReportFlagValues.Report, bput.transferReportFlagValues.ReportPath, bput.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return errors.Wrap(err, "failed to create transfer report manager")
	}
	defer bput.transferReportManager.Release()

	// set default key for encryption
	if len(bput.encryptionFlagValues.Key) == 0 {
		bput.encryptionFlagValues.Key = bput.account.Password
	}

	// parallel job manager
	ioSession := bput.filesystem.GetIOSession()
	bput.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), bput.progressFlagValues.ShowProgress, bput.progressFlagValues.ShowFullPath)
	bput.parallelPostProcessJobManager = parallel.NewParallelJobManager(1, bput.progressFlagValues.ShowProgress, bput.progressFlagValues.ShowFullPath)

	// run
	if len(bput.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = bput.ensureTargetIsDir(bput.targetPath)
		if err != nil {
			return errors.Wrapf(err, "target path %q is not a directory", bput.targetPath)
		}
	}

	// bundle manager
	bput.stagingPath = bput.bundleTransferFlagValues.IRODSTempPath
	if len(bput.stagingPath) == 0 {
		bput.stagingPath = bundle.GetStagingDirInTargetPath(bput.filesystem, bput.targetPath)
	}

	stagingDirMade, stagingDirErr := bundle.EnsureStagingDirPath(bput.filesystem, bput.stagingPath)
	if stagingDirErr != nil {
		return errors.Wrapf(stagingDirErr, "failed to prepare staging path %q", bput.stagingPath)
	}

	if stagingDirMade {
		// delete the staging directory if it was created
		defer bput.filesystem.RemoveDir(bput.stagingPath, true, true)
	}

	bput.bundleManager = bundle.NewBundleManager(bput.bundleTransferFlagValues.MinFileNumInBundle, bput.bundleTransferFlagValues.MaxFileNumInBundle, bput.bundleTransferFlagValues.MaxBundleFileSize, bput.bundleTransferFlagValues.LocalTempPath, bput.stagingPath)

	// clear local bundles
	if bput.bundleTransferFlagValues.ClearOld {
		logger.Debugf("clearing a local temp directory %q", bput.bundleTransferFlagValues.LocalTempPath)
		clearErr := bput.bundleManager.ClearLocalBundles()
		if err != nil {
			return errors.Wrapf(clearErr, "failed to clear local bundle files")
		}

		logger.Debugf("clearing an irods temp directory %q", bput.stagingPath)
		err = bput.bundleManager.ClearIRODSBundles(bput.filesystem, false)
		if err != nil {
			return errors.Wrapf(err, "failed to clear irods bundle files in %q", bput.stagingPath)
		}
	}

	// this only schedules jobs, does not run them
	for _, sourcePath := range bput.sourcePaths {
		err = bput.putOne(sourcePath, bput.targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to bundle-put %q to %q", sourcePath, bput.targetPath)
		}
	}

	// process bput
	err = bput.bput()
	if err != nil {
		return errors.Wrap(err, "failed to bundle-put files")
	}

	// delete on success
	if bput.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range bput.sourcePaths {
			logger.Infof("deleting source file or directory under %q after upload", sourcePath)

			err = bput.deleteOnSuccessOne(sourcePath)
			if err != nil {
				return errors.Wrapf(err, "failed to delete source %q after upload", sourcePath)
			}
		}
	}

	// delete extra
	if bput.syncFlagValues.Delete {
		logger.Infof("deleting extra data objects and collections under %q", bput.targetPath)

		err = bput.deleteExtraOne(bput.targetPath)
		if err != nil {
			return errors.Wrap(err, "failed to delete extra data objects or collections")
		}
	}

	logger.Info("done scheduling jobs, starting jobs")

	transferErr := bput.parallelTransferJobManager.Start()
	if transferErr != nil {
		// error occurred while transferring files
		bput.parallelPostProcessJobManager.CancelJobs()
	}

	postProcessErr := bput.parallelPostProcessJobManager.Start()

	if transferErr != nil {
		return errors.Wrapf(transferErr, "failed to perform transfer jobs")
	}

	if postProcessErr != nil {
		return errors.Wrap(postProcessErr, "failed to perform post process jobs")
	}

	// print final summary
	if bput.progressFlagValues.ShowProgress {
		timeTaken := time.Since(bput.startTime).Seconds()
		totalUploadedSize := types.SizeString(bput.totalUploadedBytes)
		bps := float64(bput.totalUploadedBytes) / timeTaken
		bpsString := fmt.Sprintf("%s/s", types.SizeString(int64(bps)))
		terminal.Printf("Uploaded %d files, %s in total, time taken: %.2f seconds, average speed: %s\n", bput.totalUploadedFiles, totalUploadedSize, timeTaken, bpsString)
	}

	return nil
}

func (bput *BputCommand) bput() error {
	// seal incomplete bundle
	bput.bundleManager.DoneScheduling()

	// process bundles
	bundles := bput.bundleManager.GetBundles()
	for _, bundle := range bundles {
		if bundle.IsEmpty() {
			continue
		}

		if !bundle.IsSealed() {
			return errors.Errorf("bundle %d (%q) is not sealed, cannot process", bundle.GetID(), bundle.GetBundleFilename())
		}

		if bundle.RequireTar() {
			bput.scheduleBundleTransfer(bundle)
		} else {
			for _, bundleEntry := range bundle.GetEntries() {
				bput.scheduleBundleEntryTransfer(&bundleEntry)
			}
		}
	}

	return nil
}

func (bput *BputCommand) ensureTargetIsDir(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := bput.account.ClientZone
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := bput.filesystem.Stat(targetPath)
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

func (bput *BputCommand) getEncryptionMode(targetPath string, parentEncryptionMode encryption.EncryptionMode) encryption.EncryptionMode {
	if bput.encryptionFlagValues.Encryption {
		return bput.encryptionFlagValues.Mode
	}

	if bput.encryptionFlagValues.NoEncryption {
		return encryption.EncryptionModeNone
	}

	if !bput.encryptionFlagValues.IgnoreMeta {
		// load encryption config from meta
		targetDir := targetPath

		targetEntry, err := bput.filesystem.Stat(targetPath)
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

		encryptionConfig := encryption.GetEncryptionConfigFromMeta(bput.filesystem, targetDir)
		return encryptionConfig.Mode
	}

	return parentEncryptionMode
}

func (bput *BputCommand) putOne(sourcePath string, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := bput.account.ClientZone
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
		if !bput.noRootFlagValues.NoRoot {
			targetPath = commons_path.MakeIRODSTargetFilePath(bput.filesystem, sourcePath, targetPath)
		}

		return bput.putDir(sourceStat, sourcePath, targetPath, encryption.EncryptionModeNone)
	}

	// file
	encryptionMode := bput.getEncryptionMode(targetPath, encryption.EncryptionModeNone)
	if encryptionMode != encryption.EncryptionModeNone {
		// encrypt filename
		tempPath, err := bput.getLocalPathForEncryption(sourcePath)
		if err != nil {
			return errors.Wrapf(err, "failed to get encryption path for %q", sourcePath)
		}

		newTargetPath := path.Join(path.Dir(targetPath), path.Base(tempPath))

		return bput.putFile(sourceStat, sourcePath, tempPath, newTargetPath, encryptionMode)
	}

	targetPath = commons_path.MakeIRODSTargetFilePath(bput.filesystem, sourcePath, targetPath)
	return bput.putFile(sourceStat, sourcePath, "", targetPath, encryption.EncryptionModeNone)
}

func (bput *BputCommand) deleteOnSuccessOne(sourcePath string) error {
	sourcePath = commons_path.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceStat.IsDir() {
		// dir
		return bput.deleteDirOnSuccess(sourcePath)
	}

	// file
	return bput.deleteFileOnSuccess(sourcePath)
}

func (bput *BputCommand) deleteExtraOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := bput.account.ClientZone
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if targetEntry.IsDir() {
		// dir
		return bput.deleteExtraDir(targetPath)
	}

	// file
	return bput.deleteExtraFile(targetPath)
}

func (bput *BputCommand) scheduleBundleTransfer(bun *bundle.Bundle) {
	tarballPath := path.Join(bput.bundleManager.GetLocalTempDirPath(), bun.GetBundleFilename())
	stagingTargetPath := path.Join(bput.bundleManager.GetIRODSStagingDirPath(), bun.GetBundleFilename())

	logger := log.WithFields(log.Fields{
		"bundle_id":          bun.GetID(),
		"bundle_name":        bun.GetBundleFilename(),
		"target_path":        bun.GetIRODSDir(),
		"local_tarball_path": tarballPath,
		"staging_path":       stagingTargetPath,
	})

	defaultNotes := []string{"bput"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file", bun.GetBundleFilename())

		for _, bundleEntry := range bun.GetEntries() {
			reportFile := &transfer.TransferReportFile{
				Method:     transfer.TransferMethodBput,
				StartAt:    now,
				EndAt:      now,
				SourcePath: bundleEntry.LocalPath,
				SourceSize: bundleEntry.Size,
				DestPath:   bundleEntry.IRODSPath,
				Error:      err,
				Notes:      newNotes,
			}

			bput.transferReportManager.AddFile(reportFile)
		}
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file", bun.GetBundleFilename())

		bput.transferReportManager.AddTransfer(result, transfer.TransferMethodBput, err, newNotes)

		startTime := result.StartTime
		endTime := result.EndTime

		for _, bundleEntry := range bun.GetEntries() {
			reportFile := &transfer.TransferReportFile{
				Method:     transfer.TransferMethodBput,
				StartAt:    startTime,
				EndAt:      endTime,
				SourcePath: bundleEntry.LocalPath,
				SourceSize: bundleEntry.Size,
				DestPath:   bundleEntry.IRODSPath,
				Error:      err,
				Notes:      newNotes,
			}

			bput.transferReportManager.AddFile(reportFile)
		}
	}

	_, threadsRequired := bput.determineTransferMethodForBundle(bun)

	// task for bundling and uploading
	bundleTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("upload", -1, bun.GetSize(), true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for bundle transfer")
			return nil
		}

		logger.Debug("creating a tarball")

		notes := []string{}

		// create a bundle file
		job.Progress("bundle", 0, bun.GetSize(), false)
		tarball := bundle.NewTar(bun.GetIRODSDir())

		for _, bundleEntry := range bun.GetEntries() {
			// encrypt if needed
			if bundleEntry.EncryptionMode != encryption.EncryptionModeNone {
				notes = append(notes, "encrypt")

				_, encryptErr := bput.encryptFile(bundleEntry.LocalPath, bundleEntry.TempPath, bundleEntry.EncryptionMode)
				if encryptErr != nil {
					job.Progress("bundle", -1, bun.GetSize(), true)

					reportSimple(encryptErr, notes...)
					return errors.Wrapf(encryptErr, "failed to encrypt file %s", bundleEntry.LocalPath)
				}

				tarball.AddEntry(bundleEntry.TempPath, bundleEntry.IRODSPath)
			} else {
				tarball.AddEntry(bundleEntry.LocalPath, bundleEntry.IRODSPath)
			}
		}

		tarErr := tarball.CreateTarball(tarballPath, nil)
		if tarErr != nil {
			job.Progress("bundle", -1, bun.GetSize(), true)

			reportSimple(tarErr, "tar")
			return errors.Wrapf(tarErr, "failed to create a tarball %q for bundle %d", tarballPath, bun.GetID())
		}
		defer os.Remove(tarballPath)

		job.Progress("bundle", bun.GetSize(), bun.GetSize(), false)
		logger.Debug("created a tarball")

		tarballStat, tarErr := os.Stat(tarballPath)
		if tarErr != nil {
			job.Progress("bundle", -1, bun.GetSize(), true)

			reportSimple(tarErr, "tar")
			return errors.Wrapf(tarErr, "failed to create a tarball %q for bundle %d", tarballPath, bun.GetID())
		}

		// tarball size
		job.Progress("upload", 0, tarballStat.Size(), false)

		parentTargetPath := path.Dir(bun.GetIRODSDir())
		_, statErr := bput.filesystem.Stat(parentTargetPath)
		if statErr != nil {
			// must exist, mkdir is performed at putDir
			job.Progress("upload", -1, tarballStat.Size(), true)

			reportSimple(statErr)
			return errors.Wrapf(statErr, "failed to stat %q", parentTargetPath)
		}

		notes = append(notes, fmt.Sprintf("staging path %q", stagingTargetPath))

		progressCallbackPut := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		uploadResult, uploadErr := bput.filesystem.UploadFileParallel(tarballPath, stagingTargetPath, "", threadsRequired, false, bput.checksumFlagValues.VerifyChecksum, false, progressCallbackPut)

		notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))

		if uploadErr != nil {
			job.Progress("upload", -1, tarballStat.Size(), true)
			job.Progress("checksum", -1, tarballStat.Size(), true)

			reportTransfer(uploadResult, uploadErr, notes...)
			return errors.Wrapf(uploadErr, "failed to upload a tarball %q to %q", tarballPath, stagingTargetPath)
		}

		reportTransfer(uploadResult, nil, notes...)

		logger.Debug("uploaded a tarball")

		// extract the bundle in iRODS
		logger.Debug("extracting a tarball")

		job.Progress("extract", 0, tarballStat.Size(), false)

		extractErr := bput.filesystem.ExtractStructFile(stagingTargetPath, bun.GetIRODSDir(), "", irodsclient_types.TAR_FILE_DT, bput.forceFlagValues.Force, !bput.bundleTransferFlagValues.NoBulkRegistration)
		if extractErr != nil {
			job.Progress("extract", -1, tarballStat.Size(), true)

			reportSimple(extractErr, "extract")
			return errors.Wrapf(extractErr, "failed to extract a tarball %q to %q", stagingTargetPath, bun.GetIRODSDir())
		}

		logger.Debug("extracted a tarball")

		// remove the tarball
		logger.Debug("removing a tarball")
		removeErr := bput.filesystem.RemoveFile(stagingTargetPath, true)
		if removeErr != nil {
			job.Progress("extract", -1, tarballStat.Size(), true)
			reportSimple(removeErr, "remove")
			return errors.Wrapf(removeErr, "failed to remove a tarball %q", stagingTargetPath)
		}

		job.Progress("extract", tarballStat.Size(), tarballStat.Size(), false)

		logger.Debug("removed a tarball")

		bput.totalUploadedFiles += bun.GetEntryNumber()
		bput.totalUploadedBytes += bun.GetSize()

		return nil
	}

	bput.parallelTransferJobManager.Schedule(bun.GetBundleFilename(), bundleTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a bundle file upload (with %d files), %d threads", bun.GetEntryNumber(), threadsRequired)
}

func (bput *BputCommand) scheduleBundleEntryTransfer(bundleEntry *bundle.BundleEntry) {
	logger := log.WithFields(log.Fields{
		"bundle_entry": bundleEntry.IRODSPath,
		"local_path":   bundleEntry.LocalPath,
		"temp_path":    bundleEntry.TempPath,
		"encryption":   bundleEntry.EncryptionMode,
	})

	defaultNotes := []string{"bput", "no bundle"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    now,
			EndAt:      now,
			SourcePath: bundleEntry.LocalPath,
			SourceSize: bundleEntry.Size,
			DestPath:   bundleEntry.IRODSPath,
			Error:      err,
			Notes:      newNotes,
		}

		bput.transferReportManager.AddFile(reportFile)
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		bput.transferReportManager.AddTransfer(result, transfer.TransferMethodPut, err, newNotes)
	}

	_, threadsRequired := bput.determineTransferMethod(bundleEntry.Size)

	putTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("upload", -1, bundleEntry.Size, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for uploading")
			return nil
		}

		logger.Debug("uploading a file")

		progressCallbackPut := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		job.Progress("upload", 0, bundleEntry.Size, false)

		notes := []string{}

		// encrypt
		if bundleEntry.EncryptionMode != encryption.EncryptionModeNone {
			notes = append(notes, "encrypt")

			_, encryptErr := bput.encryptFile(bundleEntry.LocalPath, bundleEntry.TempPath, bundleEntry.EncryptionMode)
			if encryptErr != nil {
				job.Progress("upload", -1, bundleEntry.Size, true)

				reportSimple(encryptErr, notes...)
				return errors.Wrapf(encryptErr, "failed to encrypt file %s", bundleEntry.LocalPath)
			}

			defer func() {
				if len(bundleEntry.TempPath) > 0 {
					// remove temp file
					logger.Debug("removing a temporary file")
					os.Remove(bundleEntry.TempPath)
				}
			}()
		}

		uploadSourcePath := bundleEntry.LocalPath
		if len(bundleEntry.TempPath) > 0 {
			uploadSourcePath = bundleEntry.TempPath
		}

		parentTargetPath := path.Dir(bundleEntry.IRODSPath)
		_, statErr := bput.filesystem.Stat(parentTargetPath)
		if statErr != nil {
			// must exist, mkdir is performed at putDir
			job.Progress("upload", -1, bundleEntry.Size, true)

			reportSimple(statErr)
			return errors.Wrapf(statErr, "failed to stat %q", parentTargetPath)
		}

		uploadResult, uploadErr := bput.filesystem.UploadFileParallel(uploadSourcePath, bundleEntry.IRODSPath, "", threadsRequired, false, bput.checksumFlagValues.VerifyChecksum, false, progressCallbackPut)
		notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))

		if uploadErr != nil {
			job.Progress("upload", -1, bundleEntry.Size, true)
			job.Progress("checksum", -1, bundleEntry.Size, true)

			reportTransfer(uploadResult, uploadErr, notes...)
			return errors.Wrapf(uploadErr, "failed to upload %q to %q", bundleEntry.LocalPath, bundleEntry.IRODSPath)
		}

		reportTransfer(uploadResult, nil, notes...)

		logger.Debug("uploaded a file")

		return nil
	}

	bput.parallelTransferJobManager.Schedule(bundleEntry.LocalPath, putTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a file upload, %d threads", threadsRequired)
}

func (bput *BputCommand) schedulePut(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, encryptionMode encryption.EncryptionMode) error {
	// add to bundle
	bundleEntry := bundle.BundleEntry{
		LocalPath:      sourcePath,
		TempPath:       tempPath,
		IRODSPath:      targetPath,
		Size:           sourceStat.Size(),
		EncryptionMode: encryptionMode,
	}

	bundleErr := bput.bundleManager.Add(bundleEntry)
	if bundleErr != nil {
		return errors.Wrapf(bundleErr, "failed to add %q to bundle", sourcePath)
	}

	return nil
}

func (bput *BputCommand) scheduleDeleteFileOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"bput", "delete on success", "file"}

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

		bput.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting empty directory")
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

	bput.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled a file deletion")
}

func (bput *BputCommand) scheduleDeleteDirOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"bput", "delete on success", "directory"}

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

		bput.transferReportManager.AddFile(reportFile)
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

	bput.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an empty directory deletion")
}

func (bput *BputCommand) scheduleDeleteExtraFile(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"bput", "extra", "file"}

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

		bput.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting an extra data object")
			return nil
		}

		logger.Debug("deleting an extra data object")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := bput.filesystem.RemoveFile(targetPath, true)
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

	bput.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra data object deletion")
}

func (bput *BputCommand) scheduleDeleteExtraDir(targetPath string) {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"bput", "extra", "directory"}

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

		bput.transferReportManager.AddFile(reportFile)
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
		removeErr := bput.filesystem.RemoveDir(targetPath, false, false)
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

	bput.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled an extra collection deletion")
}

func (bput *BputCommand) putFile(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, encryptionMode encryption.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"source_path":     sourcePath,
		"temp_path":       tempPath,
		"target_path":     targetPath,
		"encryption_mode": encryptionMode,
	})

	defaultNotes := []string{"bput"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodBput,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		bput.transferReportManager.AddFile(reportFile)
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

		bput.transferReportManager.AddFile(reportFile)
	}

	bput.mutex.Lock()
	commons_path.MarkIRODSPathMap(bput.updatedPathMap, targetPath)
	bput.mutex.Unlock()

	if bput.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip uploading a file %q to %q. The file is hidden!\n", sourcePath, targetPath)
			logger.Debug("skip uploading a file. The file is hidden!")
			return nil
		}
	}

	if bput.syncFlagValues.Age > 0 {
		// exclude old
		age := time.Since(sourceStat.ModTime())
		maxAge := time.Duration(bput.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			reportSimple(nil, "age", "skipped")
			terminal.Printf("skip uploading a file %q to %q. The file is too old (%s > %s)!\n", sourcePath, targetPath, age, maxAge)
			logger.Debugf("skip uploading a file. The file is too old (age %s > max_age %s)!", age, maxAge)
			return nil
		}
	}

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			bput.schedulePut(sourceStat, sourcePath, tempPath, targetPath, encryptionMode)
			return nil
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if bput.syncFlagValues.Sync {
			// if it is sync, remove
			if bput.forceFlagValues.Force {
				startTime := time.Now()
				removeErr := bput.filesystem.RemoveDir(targetPath, true, true)
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
					removeErr := bput.filesystem.RemoveDir(targetPath, true, true)
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

	if bput.differentialTransferFlagValues.DifferentialTransfer {
		if bput.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceStat.Size() {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                transfer.TransferMethodBput,
					StartAt:               now,
					EndAt:                 now,
					SourcePath:            sourcePath,
					SourceSize:            sourceStat.Size(),
					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),

					Notes: []string{"bput", "file", "differential", "no_hash", "same size", "skipped"},
				}

				bput.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip uploading a file %q to %q. The file already exists!\n", sourcePath, targetPath)
				logger.Debug("skip uploading a file. The file already exists!")
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
							Method:                  transfer.TransferMethodBput,
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

							Notes: []string{"bput", "file", "differential", "same checksum", "skipped"},
						}

						bput.transferReportManager.AddFile(reportFile)

						terminal.Printf("skip uploading a file %q to %q. The data object with the same hash already exists!\n", sourcePath, targetPath)
						logger.Debug("skip uploading a file. The data object with the same hash already exists!")
						return nil
					}
				}
			}
		}
	} else {
		if !bput.forceFlagValues.Force {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Data object %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                transfer.TransferMethodBput,
					StartAt:               now,
					EndAt:                 now,
					SourcePath:            sourcePath,
					SourceSize:            sourceStat.Size(),
					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksum:          hex.EncodeToString(targetEntry.CheckSum),
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),

					Notes: []string{"bput", "file", "overwrite", "declined", "skipped"},
				}

				bput.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip uploading a file %q to %q. The data object already exists!\n", sourcePath, targetPath)
				logger.Debug("skip uploading a file. The data object already exists!")
				return nil
			}
		}
	}

	// schedule
	bput.schedulePut(sourceStat, sourcePath, tempPath, targetPath, encryptionMode)
	return nil
}

func (bput *BputCommand) putDir(sourceStat fs.FileInfo, sourcePath string, targetPath string, parentEncryptionMode encryption.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"source_path":            sourcePath,
		"target_path":            targetPath,
		"parent_encryption_mode": parentEncryptionMode,
	})

	defaultNotes := []string{"bput", "directory"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodBput,
			StartAt:    startTime,
			EndAt:      endTime,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		bput.transferReportManager.AddFile(reportFile)
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

		bput.transferReportManager.AddFile(reportFile)
	}

	bput.mutex.Lock()
	commons_path.MarkIRODSPathMap(bput.updatedPathMap, targetPath)
	bput.mutex.Unlock()

	if bput.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip uploading a directory %q to %q. The directory is hidden!\n", sourcePath, targetPath)
			logger.Debug("skip uploading a directory. The directory is hidden!")
			return nil
		}
	}

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			startTime := time.Now()
			err = bput.filesystem.MakeDir(targetPath, true)
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
			if bput.syncFlagValues.Sync {
				// if it is sync, remove
				if bput.forceFlagValues.Force {
					startTime := time.Now()
					removeErr := bput.filesystem.RemoveFile(targetPath, true)
					endTime := time.Now()
					reportOverwrite(startTime, endTime, removeErr)

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to put entries
				} else {
					// ask
					overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a collection %q, but data object exists. Overwrite?", targetPath))
					if overwrite {
						startTime := time.Now()
						removeErr := bput.filesystem.RemoveFile(targetPath, true)
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
						terminal.Printf("skip uploading a directory %q to %q. The data object already exists!\n", sourcePath, targetPath)
						logger.Debug("skip uploading a directory. The data object already exists!")
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
	encryptionMode := bput.getEncryptionMode(targetPath, parentEncryptionMode)

	// get entries
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a directory %q", sourcePath)
	}

	for _, entry := range entries {
		newEntryPath := commons_path.MakeIRODSTargetFilePath(bput.filesystem, entry.Name(), targetPath)

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
			err = bput.putDir(entryStat, entryPath, newEntryPath, encryptionMode)
			if err != nil {
				return err
			}
		} else {
			// file
			if encryptionMode != encryption.EncryptionModeNone {
				// encrypt filename
				tempPath, err := bput.getLocalPathForEncryption(entryPath)
				if err != nil {
					reportSimple(err)
					return errors.Wrapf(err, "failed to get encryption path for %q", entryPath)
				}

				newTargetPath := path.Join(path.Dir(newEntryPath), path.Base(tempPath))

				err = bput.putFile(entryStat, entryPath, tempPath, newTargetPath, encryptionMode)
				if err != nil {
					return err
				}
			} else {
				err = bput.putFile(entryStat, entryPath, "", newEntryPath, encryptionMode)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (bput *BputCommand) deleteFileOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"bput", "delete on success", "file"}

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

		bput.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a file after upload")

	if bput.forceFlagValues.Force {
		bput.scheduleDeleteFileOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a file %q after upload. Remove?", sourcePath))
		if overwrite {
			bput.scheduleDeleteFileOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (bput *BputCommand) deleteDirOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourcePath,
	})

	defaultNotes := []string{"bput", "delete on success", "directory"}

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

		bput.transferReportManager.AddFile(reportFile)
	}

	logger.Debug("removing a directory after upload")

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
			err = bput.deleteDirOnSuccess(newSourcePath)
			if err != nil {
				return err
			}
		} else {
			// file
			newSourcePath := filepath.Join(sourcePath, entry.Name())
			err = bput.deleteFileOnSuccess(newSourcePath)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	if bput.forceFlagValues.Force {
		bput.scheduleDeleteDirOnSuccess(sourcePath)
		return nil
	} else {
		// ask
		overwrite := terminal.InputYN(fmt.Sprintf("Removing a directory %q after upload. Remove?", sourcePath))
		if overwrite {
			bput.scheduleDeleteDirOnSuccess(sourcePath)
			return nil
		} else {
			// do not remove
			reportSimple(nil, "declined", "skipped")
			return nil
		}
	}
}

func (bput *BputCommand) deleteExtraFile(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"bput", "extra", "file"}

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

		bput.transferReportManager.AddFile(reportFile)
	}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		report(now, now, err, additionalNotes...)
	}

	bput.mutex.RLock()
	isExtra := false
	if _, ok := bput.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	bput.mutex.RUnlock()

	if isExtra {
		// extra file
		logger.Debug("removing an extra data object")

		if bput.forceFlagValues.Force {
			bput.scheduleDeleteExtraFile(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra data object %q. Remove?", targetPath))
			if overwrite {
				bput.scheduleDeleteExtraFile(targetPath)
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

func (bput *BputCommand) deleteExtraDir(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	defaultNotes := []string{"bput", "extra", "directory"}

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

		bput.transferReportManager.AddFile(reportFile)
	}

	// scan recursively
	entries, err := bput.filesystem.List(targetPath)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a collection %q", targetPath)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// dir
			err = bput.deleteExtraDir(entry.Path)
			if err != nil {
				return err
			}
		} else {
			// file
			err = bput.deleteExtraFile(entry.Path)
			if err != nil {
				return err
			}
		}
	}

	// delete the directory itself
	bput.mutex.RLock()
	isExtra := false
	if _, ok := bput.updatedPathMap[targetPath]; !ok {
		isExtra = true
	}
	bput.mutex.RUnlock()

	if isExtra {
		// extra dir
		logger.Debug("removing an extra collection")

		if bput.forceFlagValues.Force {
			bput.scheduleDeleteExtraDir(targetPath)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra collection %q. Remove?", targetPath))
			if overwrite {
				bput.scheduleDeleteExtraDir(targetPath)
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

func (bput *BputCommand) getEncryptionManagerForEncryption(mode encryption.EncryptionMode) *encryption.EncryptionManager {
	manager := encryption.NewEncryptionManager(mode)

	switch mode {
	case encryption.EncryptionModeWinSCP, encryption.EncryptionModePGP:
		manager.SetKey([]byte(bput.encryptionFlagValues.Key))
	case encryption.EncryptionModeSSH:
		manager.SetPublicPrivateKey(bput.encryptionFlagValues.PublicPrivateKeyPath)
	}

	return manager
}

func (bput *BputCommand) getLocalPathForEncryption(sourcePath string) (string, error) {
	if bput.encryptionFlagValues.Mode != encryption.EncryptionModeNone {
		encryptManager := bput.getEncryptionManagerForEncryption(bput.encryptionFlagValues.Mode)
		sourceFilename := filepath.Base(sourcePath)

		encryptedFilename, err := encryptManager.EncryptFilename(sourceFilename)
		if err != nil {
			return "", errors.Wrapf(err, "failed to encrypt filename %q", sourcePath)
		}

		tempFilePath := commons_path.MakeLocalTargetFilePath(encryptedFilename, bput.encryptionFlagValues.TempPath)

		return tempFilePath, nil
	}

	return "", nil
}

func (bput *BputCommand) encryptFile(sourcePath string, encryptedFilePath string, encryptionMode encryption.EncryptionMode) (bool, error) {
	logger := log.WithFields(log.Fields{
		"source_path":     sourcePath,
		"encrypted_path":  encryptedFilePath,
		"encryption_mode": encryptionMode,
	})

	if encryptionMode != encryption.EncryptionModeNone {
		logger.Debug("encrypt a file")

		encryptManager := bput.getEncryptionManagerForEncryption(encryptionMode)

		err := encryptManager.EncryptFile(sourcePath, encryptedFilePath)
		if err != nil {
			return false, errors.Wrapf(err, "failed to encrypt %q to %q", sourcePath, encryptedFilePath)
		}

		return true, nil
	}

	return false, nil
}

func (bput *BputCommand) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	threads := parallel.CalculateThreadForTransferJob(size, bput.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to upload
	if bput.parallelTransferFlagValues.SingleThread || bput.parallelTransferFlagValues.ThreadNumber <= 2 || bput.parallelTransferFlagValues.ThreadNumberPerFile == 1 || !bput.filesystem.SupportParallelUpload() {
		threads = 1
	}

	if bput.parallelTransferFlagValues.Icat {
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

func (bput *BputCommand) determineTransferMethodForBundle(bun *bundle.Bundle) (transfer.TransferMode, int) {
	threads := parallel.CalculateThreadForTransferJob(bun.GetSize(), bput.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to upload
	if bput.parallelTransferFlagValues.SingleThread || bput.parallelTransferFlagValues.ThreadNumber <= 2 || bput.parallelTransferFlagValues.ThreadNumberPerFile == 1 || !bput.filesystem.SupportParallelUpload() {
		threads = 1
	}

	if bput.parallelTransferFlagValues.Icat {
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

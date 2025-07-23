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
	"golang.org/x/xerrors"
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
	flag.SetChecksumFlags(bputCmd, true, false)
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

		updatedPathMap: map[string]bool{},
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
		return nil, xerrors.Errorf("failed to put multiple source collections without creating root directory")
	}

	return bput, nil
}

func (bput *BputCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(bput.command)
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
	bput.account = config.GetSessionConfig().ToIRODSAccount()
	bput.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(bput.account, bput.maxConnectionNum, bput.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bput.filesystem.Release()

	if bput.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(bput.filesystem, bput.commonFlagValues.Timeout)
	}

	// transfer report
	bput.transferReportManager, err = transfer.NewTransferReportManager(bput.transferReportFlagValues.Report, bput.transferReportFlagValues.ReportPath, bput.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
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
			return xerrors.Errorf("target path %q is not a directory: %w", bput.targetPath, err)
		}
	}

	// bundle manager
	bput.stagingPath = bput.bundleTransferFlagValues.IRODSTempPath
	if len(bput.stagingPath) == 0 {
		bput.stagingPath = bundle.GetStagingDirInTargetPath(bput.targetPath)
	}

	stagingDirErr := bundle.EnsureStagingDirPath(bput.filesystem, bput.stagingPath)
	if stagingDirErr != nil {
		return xerrors.Errorf("failed to prepare staging path %q: %w", bput.stagingPath, stagingDirErr)
	}

	bput.bundleManager = bundle.NewBundleManager(bput.bundleTransferFlagValues.MinFileNumInBundle, bput.bundleTransferFlagValues.MaxFileNumInBundle, bput.bundleTransferFlagValues.MaxBundleFileSize, bput.bundleTransferFlagValues.LocalTempPath, bput.stagingPath)

	// clear local bundles
	if bput.bundleTransferFlagValues.ClearOld {
		logger.Debugf("clearing a local temp directory %q", bput.bundleTransferFlagValues.LocalTempPath)
		clearErr := bput.bundleManager.ClearLocalBundles()
		if err != nil {
			return xerrors.Errorf("failed to clear local bundle files: %w", clearErr)
		}

		logger.Debugf("clearing an irods temp directory %q", bput.stagingPath)
		err = bput.bundleManager.ClearIRODSBundles(bput.filesystem, false)
		if err != nil {
			return xerrors.Errorf("failed to clear irods bundle files in %q: %w", bput.stagingPath, err)
		}
	}

	// this only schedules jobs, does not run them
	for _, sourcePath := range bput.sourcePaths {
		err = bput.putOne(sourcePath, bput.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to bundle-put %q to %q: %w", sourcePath, bput.targetPath, err)
		}
	}

	// process bput
	err = bput.bput()
	if err != nil {
		return xerrors.Errorf("failed to bundle-put files: %w", err)
	}

	// delete on success
	if bput.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range bput.sourcePaths {
			logger.Infof("deleting source file or directory under %q after upload", sourcePath)

			err = bput.deleteOnSuccessOne(sourcePath)
			if err != nil {
				return xerrors.Errorf("failed to delete source %q after upload: %w", sourcePath, err)
			}
		}
	}

	// delete extra
	if bput.syncFlagValues.Delete {
		logger.Infof("deleting extra data objects and collections under %q", bput.targetPath)

		err = bput.deleteExtraOne(bput.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra data objects or collections: %w", err)
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
		return xerrors.Errorf("failed to perform transfer jobs: %w", transferErr)
	}

	if postProcessErr != nil {
		return xerrors.Errorf("failed to perform post process jobs: %w", err)
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
			return xerrors.Errorf("bundle %d (%q) is not sealed, cannot process", bundle.GetID(), bundle.GetBundleFilename())
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

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
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

		if encryptionConfig.Mode == encryption.EncryptionModeNone {
			if bput.encryptionFlagValues.Mode != encryption.EncryptionModeNone {
				return encryption.EncryptionModeNone
			}

			return bput.encryptionFlagValues.Mode
		}

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

		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
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
			return xerrors.Errorf("failed to get encryption path for %q: %w", sourcePath, err)
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
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
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
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if targetEntry.IsDir() {
		// dir
		return bput.deleteExtraDir(targetPath)
	}

	// file
	return bput.deleteExtraFile(targetPath)
}

func (bput *BputCommand) scheduleBundleTransfer(bun *bundle.Bundle) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleBundleTransfer",
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

	tarballPath := path.Join(bput.bundleManager.GetLocalTempDirPath(), bun.GetBundleFilename())
	stagingTargetPath := path.Join(bput.bundleManager.GetIRODSStagingDirPath(), bun.GetBundleFilename())

	_, threadsRequired := bput.determineTransferMethod(bun.GetSize())

	// task for bundling and uploading
	bundleTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress(-1, bun.GetSize(), true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for bundle transfer %q  %q", bun.GetBundleFilename(), bun.GetIRODSDir())
			return nil
		}

		logger.Debugf("bundling files in a bundle %d to %q", bun.GetID(), tarballPath)

		progressCallbackPut := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		notes := []string{}

		// create a bundle file
		tarball := bundle.NewTar()

		for _, bundleEntry := range bun.GetEntries() {
			// encrypt if needed
			if bundleEntry.EncryptionMode != encryption.EncryptionModeNone {
				notes = append(notes, "encrypt")

				_, encryptErr := bput.encryptFile(bundleEntry.LocalPath, bundleEntry.TempPath, bundleEntry.EncryptionMode)
				if encryptErr != nil {
					job.Progress(-1, bun.GetSize(), true)

					reportSimple(encryptErr, notes...)
					return xerrors.Errorf("failed to encrypt file %s: %w", bundleEntry.LocalPath, encryptErr)
				}

				tarball.AddEntry(bundleEntry.TempPath, bundleEntry.IRODSPath)
			} else {
				tarball.AddEntry(bundleEntry.LocalPath, bundleEntry.IRODSPath)
			}
		}

		logger.Debugf("making a bundle file %q", tarballPath)

		tarErr := tarball.CreateTarball(tarballPath, nil)
		if tarErr != nil {
			job.Progress(-1, bun.GetSize(), true)

			reportSimple(tarErr, "tar")
			return xerrors.Errorf("failed to create a tarball %q for bundle %d: %w", tarballPath, bun.GetID(), tarErr)
		}
		defer os.Remove(tarballPath)

		tarballStat, tarErr := os.Stat(tarballPath)
		if tarErr != nil {
			job.Progress(-1, bun.GetSize(), true)

			reportSimple(tarErr, "tar")
			return xerrors.Errorf("failed to create a tarball %q for bundle %d: %w", tarballPath, bun.GetID(), tarErr)
		}

		// tarball size
		job.Progress(0, tarballStat.Size(), false)

		parentTargetPath := path.Dir(bun.GetIRODSDir())
		_, statErr := bput.filesystem.Stat(parentTargetPath)
		if statErr != nil {
			// must exist, mkdir is performed at putDir
			job.Progress(-1, bun.GetSize(), true)

			reportSimple(statErr)
			return xerrors.Errorf("failed to stat %q: %w", parentTargetPath, statErr)
		}

		notes = append(notes, fmt.Sprintf("staging path %q", stagingTargetPath))

		uploadResult, uploadErr := bput.filesystem.UploadFileParallel(tarballPath, stagingTargetPath, "", threadsRequired, false, bput.checksumFlagValues.CalculateChecksum, bput.checksumFlagValues.VerifyChecksum, false, progressCallbackPut)
		notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))

		if uploadErr != nil {
			job.Progress(-1, bun.GetSize(), true)

			reportTransfer(uploadResult, uploadErr, notes...)
			return xerrors.Errorf("failed to upload a bundle %q to %q: %w", tarballPath, stagingTargetPath, uploadErr)
		}

		reportTransfer(uploadResult, nil, notes...)

		logger.Debugf("uploaded a bundle %q to %q", tarballPath, stagingTargetPath)

		job.Progress(bun.GetSize(), bun.GetSize(), false)

		// extract the bundle in iRODS
		logger.Debugf("extracting a bundle %q to %q", stagingTargetPath, bun.GetIRODSDir())

		bput.filesystem.ExtractStructFile(stagingTargetPath, bun.GetIRODSDir(), "", irodsclient_types.TAR_FILE_DT, bput.forceFlagValues.Force, !bput.bundleTransferFlagValues.NoBulkRegistration)

		return nil
	}

	bput.parallelTransferJobManager.Schedule(bun.GetBundleFilename(), bundleTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a bundle file upload %q (%d files) for iRODS directory %q, %d threads", bun.GetBundleFilename(), bun.GetEntryNumber(), bun.GetIRODSDir(), threadsRequired)
}

func (bput *BputCommand) scheduleBundleEntryTransfer(bundleEntry *bundle.BundleEntry) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleBundleEntryTransfer",
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
			job.Progress(-1, bundleEntry.Size, true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for uploading %q to %q", bundleEntry.LocalPath, bundleEntry.IRODSPath)
			return nil
		}

		logger.Debugf("uploading a file %q to %q", bundleEntry.LocalPath, bundleEntry.IRODSPath)

		progressCallbackPut := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, bundleEntry.Size, false)

		notes := []string{}

		// encrypt
		if bundleEntry.EncryptionMode != encryption.EncryptionModeNone {
			notes = append(notes, "encrypt")

			_, encryptErr := bput.encryptFile(bundleEntry.LocalPath, bundleEntry.TempPath, bundleEntry.EncryptionMode)
			if encryptErr != nil {
				job.Progress(-1, bundleEntry.Size, true)

				reportSimple(encryptErr, notes...)
				return xerrors.Errorf("failed to encrypt file: %w", encryptErr)
			}

			defer func() {
				if len(bundleEntry.TempPath) > 0 {
					// remove temp file
					logger.Debugf("removing a temporary file %q", bundleEntry.TempPath)
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
			job.Progress(-1, bundleEntry.Size, true)

			reportSimple(statErr)
			return xerrors.Errorf("failed to stat %q: %w", parentTargetPath, statErr)
		}

		uploadResult, uploadErr := bput.filesystem.UploadFileParallel(uploadSourcePath, bundleEntry.IRODSPath, "", threadsRequired, false, bput.checksumFlagValues.CalculateChecksum, bput.checksumFlagValues.VerifyChecksum, false, progressCallbackPut)
		notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))

		if uploadErr != nil {
			job.Progress(-1, bundleEntry.Size, true)

			reportTransfer(uploadResult, uploadErr, notes...)
			return xerrors.Errorf("failed to upload %q to %q: %w", bundleEntry.LocalPath, bundleEntry.IRODSPath, uploadErr)
		}

		reportTransfer(uploadResult, nil, notes...)

		logger.Debugf("uploaded a file %q to %q", bundleEntry.LocalPath, bundleEntry.IRODSPath)

		job.Progress(bundleEntry.Size, bundleEntry.Size, false)

		return nil
	}

	bput.parallelTransferJobManager.Schedule(bundleEntry.LocalPath, putTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a file upload %q to %q, %d threads", bundleEntry.LocalPath, bundleEntry.IRODSPath, threadsRequired)
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
		return xerrors.Errorf("failed to add %q to bundle: %w", sourcePath, bundleErr)
	}

	return nil
}

func (bput *BputCommand) scheduleDeleteFileOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleDeleteFileOnSuccess",
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
			job.Progress(-1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for deleting empty directory %q", sourcePath)
			return nil
		}

		logger.Debugf("deleting a file %q", sourcePath)

		job.Progress(0, 1, false)

		removeErr := os.Remove(sourcePath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress(-1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", sourcePath, removeErr)
		}

		logger.Debugf("deleted a file %q", sourcePath)
		job.Progress(1, 1, false)
		return nil
	}

	bput.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debugf("scheduled a file deletion %q", sourcePath)
}

func (bput *BputCommand) scheduleDeleteDirOnSuccess(sourcePath string) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleDeleteDirOnSuccess",
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
			job.Progress(-1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for deleting empty directory %q", sourcePath)
			return nil
		}

		logger.Debugf("deleting an empty directory %q", sourcePath)

		job.Progress(0, 1, false)

		removeErr := os.Remove(sourcePath)
		reportSimple(removeErr)

		if removeErr != nil {
			job.Progress(-1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", sourcePath, removeErr)
		}

		logger.Debugf("deleted an empty directory %q", sourcePath)
		job.Progress(1, 1, false)
		return nil
	}

	bput.parallelPostProcessJobManager.Schedule("removing - "+sourcePath, deleteTask, 1, progress.UnitsDefault)
	logger.Debugf("scheduled an empty directory deletion %q", sourcePath)
}

func (bput *BputCommand) scheduleDeleteExtraFile(targetPath string) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleDeleteExtraFile",
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
			job.Progress(-1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for deleting extra data object %q", targetPath)
			return nil
		}

		logger.Debugf("deleting an extra data object %q", targetPath)

		job.Progress(0, 1, false)

		startTime := time.Now()
		removeErr := bput.filesystem.RemoveFile(targetPath, true)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress(-1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", targetPath, removeErr)
		}

		logger.Debugf("deleted an extra data object %q", targetPath)
		job.Progress(1, 1, false)
		return nil
	}

	bput.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debugf("scheduled an extra data object deletion %q", targetPath)
}

func (bput *BputCommand) scheduleDeleteExtraDir(targetPath string) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "scheduleDeleteExtraDir",
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
			job.Progress(-1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debugf("canceled a task for deleting extra collection %q", targetPath)
			return nil
		}

		logger.Debugf("deleting an extra collection %q", targetPath)

		job.Progress(0, 1, false)

		startTime := time.Now()
		removeErr := bput.filesystem.RemoveDir(targetPath, false, false)
		endTime := time.Now()
		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress(-1, 1, true)
			return xerrors.Errorf("failed to delete %q: %w", targetPath, removeErr)
		}

		logger.Debugf("deleted an extra collection %q", targetPath)
		job.Progress(1, 1, false)
		return nil
	}

	bput.parallelPostProcessJobManager.Schedule(targetPath, deleteTask, 1, progress.UnitsDefault)
	logger.Debugf("scheduled an extra collection deletion %q", targetPath)
}

func (bput *BputCommand) putFile(sourceStat fs.FileInfo, sourcePath string, tempPath string, targetPath string, encryptionMode encryption.EncryptionMode) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "putFile",
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
			logger.Debugf("skip uploading a file %q to %q. The file is hidden!", sourcePath, targetPath)
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
			logger.Debugf("skip uploading a file %q to %q. The file is too old (%s > %s)!", sourcePath, targetPath, age, maxAge)
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

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
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
					logger.Debugf("skip uploading a file %q to %q. Collection exists with the same name!", sourcePath, targetPath)
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
				logger.Debugf("skip uploading a file %q to %q. The file already exists!", sourcePath, targetPath)
				return nil
			}
		} else {
			if targetEntry.Size == sourceStat.Size() {
				// compare hash
				if len(targetEntry.CheckSum) > 0 {
					localChecksum, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm))
					if err != nil {
						reportSimple(err, "differential")
						return xerrors.Errorf("failed to get hash for %q: %w", sourcePath, err)
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

						terminal.Printf("skip uploading a file %q to %q. The file with the same hash already exists!\n", sourcePath, targetPath)
						logger.Debugf("skip uploading a file %q to %q. The file with the same hash already exists!", sourcePath, targetPath)
						return nil
					}
				}
			}
		}
	} else {
		if !bput.forceFlagValues.Force {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("File %q already exists. Overwrite?", targetPath))
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
				logger.Debugf("skip uploading a file %q to %q. The data object already exists!", sourcePath, targetPath)
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
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "putDir",
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
			terminal.Printf("skip uploading a dir %q to %q. The dir is hidden!\n", sourcePath, targetPath)
			logger.Debugf("skip uploading a dir %q to %q. The dir is hidden!", sourcePath, targetPath)
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
				return xerrors.Errorf("failed to make a collection %q: %w", targetPath, err)
			}

			// fallthrough to put entries
		} else {
			reportSimple(err)
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
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
					overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a directory %q, but file exists. Overwrite?", targetPath))
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
						terminal.Printf("skip uploading a dir %q to %q. The data object already exists!\n", sourcePath, targetPath)
						logger.Debugf("skip uploading a dir %q to %q. The data object already exists!", sourcePath, targetPath)
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
		return xerrors.Errorf("failed to list a directory %q: %w", sourcePath, err)
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

			return xerrors.Errorf("failed to stat %q: %w", entryPath, err)
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
					return xerrors.Errorf("failed to get encryption path for %q: %w", entryPath, err)
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

// here
func (bput *BputCommand) deleteOnSuccess(sourcePath string) error {
	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceStat.IsDir() {
		return os.RemoveAll(sourcePath)
	}

	return os.Remove(sourcePath)
}

func (bput *BputCommand) deleteFileOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "deleteFileOnSuccess",
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

	logger.Debugf("removing a file %q after upload", sourcePath)

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
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "deleteDirOnSuccess",
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

	logger.Debugf("removing a directory %q after upload", sourcePath)

	// scan recursively
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		reportSimple(err)
		return xerrors.Errorf("failed to list a directory %q: %w", sourcePath, err)
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
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "deleteExtraFile",
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
		logger.Debugf("removing an extra data object %q", targetPath)

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
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "deleteExtraDir",
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
		return xerrors.Errorf("failed to list a collection %q: %w", targetPath, err)
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
		logger.Debugf("removing an extra collection %q", targetPath)

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
			return "", xerrors.Errorf("failed to encrypt filename %q: %w", sourcePath, err)
		}

		tempFilePath := commons_path.MakeLocalTargetFilePath(encryptedFilename, bput.encryptionFlagValues.TempPath)

		return tempFilePath, nil
	}

	return "", nil
}

func (bput *BputCommand) encryptFile(sourcePath string, encryptedFilePath string, encryptionMode encryption.EncryptionMode) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "encryptFile",
	})

	if encryptionMode != encryption.EncryptionModeNone {
		logger.Debugf("encrypt a file %q to %q", sourcePath, encryptedFilePath)

		encryptManager := bput.getEncryptionManagerForEncryption(encryptionMode)

		err := encryptManager.EncryptFile(sourcePath, encryptedFilePath)
		if err != nil {
			return false, xerrors.Errorf("failed to encrypt %q to %q: %w", sourcePath, encryptedFilePath, err)
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

func (bput *BputCommand) createTarball(bun *bundle.Bundle) (string, int64, error) {
	if !bun.IsSealed() {
		return "", 0, xerrors.Errorf("bundle %d is not sealed, cannot create tarball", bun.GetID())
	}

	tar := bundle.NewTar()
	for _, entry := range bun.GetEntries() {
		addErr := tar.AddEntry(entry.LocalPath, entry.IRODSPath)
		if addErr != nil {
			return "", 0, xerrors.Errorf("failed to add entry %q to tarball: %w", entry.LocalPath, addErr)
		}
	}

	localTargetPath := path.Join(bput.bundleManager.GetLocalTempDirPath(), bun.GetBundleFilename())

	tarballErr := tar.CreateTarball(localTargetPath, nil)
	if tarballErr != nil {
		return "", 0, xerrors.Errorf("failed to create tarball for bundle %d at %s: %w", bun.GetID(), localTargetPath, tarballErr)
	}

	return localTargetPath, tar.GetSize(), nil
}

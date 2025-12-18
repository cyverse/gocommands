package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/parallel"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/transfer"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/cyverse/gocommands/commons/wildcard"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:     "cp <data-object-or-collection>... <target-data-object-or-collection>",
	Aliases: []string{"icp", "copy"},
	Short:   "Copy iRODS data objects or collections to a target data object or collection",
	Long:    `This command copies iRODS data objects or collections to the specified target data object or collection.`,
	RunE:    processCpCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddCpCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(cpCmd, false)

	flag.SetBundleTransferFlags(cpCmd, true, true)
	flag.SetParallelTransferFlags(cpCmd, true, true)
	flag.SetForceFlags(cpCmd, false)
	flag.SetRecursiveFlags(cpCmd, false)
	flag.SetProgressFlags(cpCmd)
	flag.SetRetryFlags(cpCmd)
	flag.SetDifferentialTransferFlags(cpCmd, false)
	flag.SetChecksumFlags(cpCmd)
	flag.SetNoRootFlags(cpCmd)
	flag.SetSyncFlags(cpCmd, true)
	flag.SetHiddenFileFlags(cpCmd)
	flag.SetTransferReportFlags(cpCmd)
	flag.SetWildcardSearchFlags(cpCmd)

	rootCmd.AddCommand(cpCmd)
}

func processCpCommand(command *cobra.Command, args []string) error {
	cp, err := NewCpCommand(command, args)
	if err != nil {
		return err
	}

	return cp.Process()
}

type CpCommand struct {
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
	hiddenFileFlagValues           *flag.HiddenFileFlagValues
	transferReportFlagValues       *flag.TransferReportFlagValues
	wildcardSearchFlagValues       *flag.WildcardSearchFlagValues

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

func NewCpCommand(command *cobra.Command, args []string) (*CpCommand, error) {
	cp := &CpCommand{
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
		hiddenFileFlagValues:           flag.GetHiddenFileFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),
		wildcardSearchFlagValues:       flag.GetWildcardSearchFlagValues(),

		updatedPathMap: map[string]bool{},
	}

	// path
	cp.targetPath = args[len(args)-1]
	cp.sourcePaths = args[:len(args)-1]

	if cp.noRootFlagValues.NoRoot && len(cp.sourcePaths) > 1 {
		return nil, errors.New("failed to copy multiple source collections without creating root directory")
	}

	return cp, nil
}

func (cp *CpCommand) Process() error {
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(cp.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// Create a file system
	cp.account = config.GetSessionConfig().ToIRODSAccount()
	cp.filesystem, err = irods.GetIRODSFSClient(cp.account, false, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer cp.filesystem.Release()

	if cp.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(cp.filesystem, cp.commonFlagValues.Timeout)
	}

	// transfer report
	cp.transferReportManager, err = transfer.NewTransferReportManager(cp.transferReportFlagValues.Report, cp.transferReportFlagValues.ReportPath, cp.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return errors.Wrapf(err, "failed to create transfer report manager")
	}
	defer cp.transferReportManager.Release()

	// parallel job manager
	metaSession := cp.filesystem.GetMetadataSession()
	cp.parallelTransferJobManager = parallel.NewParallelJobManager(metaSession.GetMaxConnections(), cp.progressFlagValues.ShowProgress, cp.progressFlagValues.ShowFullPath)
	cp.parallelPostProcessJobManager = parallel.NewParallelJobManager(1, cp.progressFlagValues.ShowProgress, cp.progressFlagValues.ShowFullPath)

	// Expand wildcards
	if cp.wildcardSearchFlagValues.WildcardSearch {
		cp.sourcePaths, err = wildcard.ExpandWildcards(cp.filesystem, cp.account, cp.sourcePaths, true, true)
		if err != nil {
			return errors.Wrapf(err, "failed to expand wildcards")
		}
	}

	// run
	if len(cp.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = cp.ensureTargetIsDir(cp.targetPath)
		if err != nil {
			return errors.Wrapf(err, "target path %q is not a directory", cp.targetPath)
		}
	}

	for _, sourcePath := range cp.sourcePaths {
		err = cp.copyOne(sourcePath, cp.targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to copy %q to %q", sourcePath, cp.targetPath)
		}
	}

	// delete extra
	if cp.syncFlagValues.Delete {
		logger.Infof("deleting extra data objects and collections under %q", cp.targetPath)

		err = cp.deleteExtraOne(cp.targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to delete extra data objects or collections")
		}
	}

	logger.Info("done scheduling jobs, starting jobs")

	transferErr := cp.parallelTransferJobManager.Start()
	if transferErr != nil {
		// error occurred while transferring files
		cp.parallelPostProcessJobManager.CancelJobs()
	}

	postProcessErr := cp.parallelPostProcessJobManager.Start()

	if transferErr != nil {
		return errors.Wrapf(transferErr, "failed to perform transfer jobs")
	}

	if postProcessErr != nil {
		return errors.Wrapf(postProcessErr, "failed to perform post process jobs")
	}

	return nil
}

func (cp *CpCommand) ensureTargetIsDir(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := cp.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := cp.filesystem.Stat(targetPath)
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

func (cp *CpCommand) copyOne(sourcePath string, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := cp.account.ClientZone
	sourcePath = path.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := cp.filesystem.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceEntry.IsDir() {
		// dir
		if !cp.recursiveFlagValues.Recursive {
			return errors.New("cannot copy a collection, turn on 'recurse' option")
		}

		if !cp.noRootFlagValues.NoRoot {
			targetPath = path.MakeIRODSTargetFilePath(cp.filesystem, sourcePath, targetPath)
		}

		return cp.copyDir(sourceEntry, targetPath)
	}

	// file
	targetPath = path.MakeIRODSTargetFilePath(cp.filesystem, sourcePath, targetPath)
	return cp.copyFile(sourceEntry, targetPath)
}

func (cp *CpCommand) deleteExtraOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := cp.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if targetEntry.IsDir() {
		// dir
		return cp.deleteExtraDir(targetEntry)
	}

	// file
	return cp.deleteExtraFile(targetEntry)
}

func (cp *CpCommand) scheduleCopy(sourceEntry *irodsclient_fs.Entry, targetPath string, targetEntry *irodsclient_fs.Entry) {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	defaultNotes := []string{"cp", "file"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodCopy,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
	}

	copyTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("copy", -1, 1, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for copying")
			return nil
		}

		logger.Debug("copying a data object")

		job.Progress("copy", 0, 1, false)

		startTime := time.Now()
		copyErr := cp.filesystem.CopyFileToFile(sourceEntry.Path, targetPath, true)
		endTime := time.Now()

		if copyErr != nil {
			job.Progress("copy", -1, 1, true)

			reportSimple(copyErr)
			return errors.Wrapf(copyErr, "failed to copy %q to %q", sourceEntry.Path, targetPath)
		}

		reportFile := &transfer.TransferReportFile{
			Method:                  transfer.TransferMethodCopy,
			StartAt:                 startTime,
			EndAt:                   endTime,
			SourcePath:              sourceEntry.Path,
			SourceSize:              sourceEntry.Size,
			SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
			SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
			DestPath:                targetPath,

			Notes: defaultNotes,
		}

		if targetEntry != nil {
			reportFile.DestSize = targetEntry.Size
			reportFile.DestChecksumAlgorithm = string(targetEntry.CheckSumAlgorithm)
			reportFile.DestChecksum = hex.EncodeToString(targetEntry.CheckSum)
		}

		cp.transferReportManager.AddFile(reportFile)

		logger.Debug("copied a data object")
		job.Progress("copy", 1, 1, false)

		return nil
	}

	cp.parallelTransferJobManager.Schedule(sourceEntry.Path, copyTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled a data object copy")
}

func (cp *CpCommand) scheduleDeleteExtraFile(targetEntry *irodsclient_fs.Entry) {
	logger := log.WithFields(log.Fields{
		"target_path": targetEntry.Path,
	})

	defaultNotes := []string{"cp", "extra", "file"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetEntry.Path,
			Error:    err,
			Notes:    newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting")
			return nil
		}

		logger.Debug("deleting a data object")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		removeErr := cp.filesystem.RemoveFile(targetEntry.Path, true)
		endTime := time.Now()

		report(startTime, endTime, removeErr)

		if removeErr != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(removeErr, "failed to delete %q", targetEntry.Path)
		}

		logger.Debug("deleted a data object")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	cp.parallelPostProcessJobManager.Schedule(targetEntry.Path, deleteTask, 1, progress.UnitsDefault)
	logger.Debug("scheduled a data object deletion")
}

func (cp *CpCommand) scheduleDeleteExtraDir(targetEntry *irodsclient_fs.Entry) {
	logger := log.WithFields(log.Fields{
		"target_path": targetEntry.Path,
	})

	defaultNotes := []string{"cp", "extra", "directory"}

	report := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetEntry.Path,
			Error:    err,
			Notes:    newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
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
			logger.Debug("canceled a task for deleting extra collection")
			return nil
		}

		logger.Debug("deleting an extra collection")

		job.Progress("delete", 0, 1, false)

		startTime := time.Now()
		err := cp.filesystem.RemoveDir(targetEntry.Path, true, true)
		endTime := time.Now()

		report(startTime, endTime, err)

		if err != nil {
			job.Progress("delete", -1, 1, true)
			return errors.Wrapf(err, "failed to delete %q", targetEntry.Path)
		}

		logger.Debug("deleted an extra collection")
		job.Progress("delete", 1, 1, false)
		return nil
	}

	cp.parallelPostProcessJobManager.Schedule(targetEntry.Path, deleteTask, 1, progress.UnitsDefault)
	logger.Debugf("scheduled a collection deletion %q", targetEntry.Path)
}

func (cp *CpCommand) copyFile(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	defaultNotes := []string{"cp"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodCopy,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
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

		cp.transferReportManager.AddFile(reportFile)
	}

	cp.mutex.Lock()
	path.MarkIRODSPathMap(cp.updatedPathMap, targetPath)
	cp.mutex.Unlock()

	if cp.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip copying a data object %q to %q. The data object is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debug("skip copying a data object. The data object is hidden!", sourceEntry)
			return nil
		}
	}

	if cp.syncFlagValues.Age > 0 {
		// check age
		age := time.Since(sourceEntry.ModifyTime)
		maxAge := time.Duration(cp.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			reportSimple(nil, "age", "skipped")
			terminal.Printf("skip copying a data object %q to %q. The data object is too old (%s > %s)!\n", sourceEntry.Path, targetPath, age, maxAge)
			logger.Debugf("skip copying a data object. The data object is too old (%s > %s)!", age, maxAge)
			return nil
		}
	}

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			cp.scheduleCopy(sourceEntry, targetPath, nil)
			return nil
		}

		reportSimple(err)
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if cp.syncFlagValues.Sync {
			// if it is sync, remove
			if cp.forceFlagValues.Force {
				startTime := time.Now()
				removeErr := cp.filesystem.RemoveDir(targetPath, true, true)
				endTime := time.Now()

				reportOverwrite(startTime, endTime, removeErr, "directory")

				if removeErr != nil {
					return removeErr
				}

				// fallthrough to copy
			} else {
				// ask
				overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a data object %q, but collection exists. Overwrite?", targetPath))
				if overwrite {
					startTime := time.Now()
					removeErr := cp.filesystem.RemoveDir(targetPath, true, true)
					endTime := time.Now()

					reportOverwrite(startTime, endTime, removeErr, "directory")

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to copy
				} else {
					overwriteErr := types.NewNotFileError(targetPath)
					now := time.Now()

					reportOverwrite(now, now, overwriteErr, "directory", "declined")
					return overwriteErr
				}
			}
		} else {
			notFileErr := types.NewNotFileError(targetPath)
			now := time.Now()

			reportOverwrite(now, now, notFileErr, "directory")
			return notFileErr
		}
	}

	if cp.differentialTransferFlagValues.DifferentialTransfer {
		if cp.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceEntry.Size {
				// skip
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                  transfer.TransferMethodCopy,
					StartAt:                 now,
					EndAt:                   now,
					SourcePath:              sourceEntry.Path,
					SourceSize:              sourceEntry.Size,
					SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
					DestPath:                targetPath,
					DestSize:                targetEntry.Size,
					DestChecksumAlgorithm:   string(targetEntry.CheckSumAlgorithm),
					DestChecksum:            hex.EncodeToString(targetEntry.CheckSum),

					Notes: []string{"cp", "file", "differential", "no hash", "same size", "skipped"},
				}

				cp.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip copying a data object %q to %q. The data object already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip copying a data object. The data object already exists!")
				return nil
			}
		} else {
			if targetEntry.Size == sourceEntry.Size {
				// compare hash
				if len(sourceEntry.CheckSum) > 0 && bytes.Equal(sourceEntry.CheckSum, targetEntry.CheckSum) {
					now := time.Now()
					reportFile := &transfer.TransferReportFile{
						Method:                  transfer.TransferMethodCopy,
						StartAt:                 now,
						EndAt:                   now,
						SourcePath:              sourceEntry.Path,
						SourceSize:              sourceEntry.Size,
						SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
						SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
						DestPath:                targetPath,
						DestSize:                targetEntry.Size,
						DestChecksum:            hex.EncodeToString(targetEntry.CheckSum),
						DestChecksumAlgorithm:   string(targetEntry.CheckSumAlgorithm),

						Notes: []string{"cp", "file", "differential", "same checksum", "skipped"},
					}

					cp.transferReportManager.AddFile(reportFile)

					terminal.Printf("skip copying a data object %q to %q. The data object with the same hash already exists!\n", sourceEntry.Path, targetPath)
					logger.Debugf("skip copying a data object %q to %q. The data object with the same hash already exists!", sourceEntry.Path, targetPath)
					return nil
				}
			}
		}
	} else {
		if !cp.forceFlagValues.Force {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Data object %q already exists. Overwrite?", targetPath))
			if !overwrite {
				now := time.Now()
				reportFile := &transfer.TransferReportFile{
					Method:                  transfer.TransferMethodCopy,
					StartAt:                 now,
					EndAt:                   now,
					SourcePath:              sourceEntry.Path,
					SourceSize:              sourceEntry.Size,
					SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
					SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
					DestPath:                targetPath,
					DestSize:                targetEntry.Size,
					DestChecksum:            hex.EncodeToString(targetEntry.CheckSum),
					DestChecksumAlgorithm:   string(targetEntry.CheckSumAlgorithm),

					Notes: []string{"cp", "file", "overwrite", "declined", "skipped"},
				}

				cp.transferReportManager.AddFile(reportFile)

				terminal.Printf("skip copying a data object %q to %q. The data object already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip copying a data object %q to %q. The data object already exists!", sourceEntry.Path, targetPath)
				return nil
			}
		}
	}

	// schedule
	cp.scheduleCopy(sourceEntry, targetPath, targetEntry)
	return nil
}

func (cp *CpCommand) copyDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	defaultNotes := []string{"cp", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodCopy,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
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

		cp.transferReportManager.AddFile(reportFile)
	}

	cp.mutex.Lock()
	path.MarkIRODSPathMap(cp.updatedPathMap, targetPath)
	cp.mutex.Unlock()

	if cp.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			reportSimple(nil, "hidden", "skipped")
			terminal.Printf("skip copying a collection %q to %q. The collection is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debug("skip copying a collection. The collection is hidden!")
			return nil
		}
	}

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			err = cp.filesystem.MakeDir(targetPath, true)
			reportSimple(err)
			if err != nil {
				return errors.Wrapf(err, "failed to make a collection %q", targetPath)
			}

			// fallthrough to copy entries
		} else {
			reportSimple(err)
			return errors.Wrapf(err, "failed to stat %q", targetPath)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			if cp.syncFlagValues.Sync {
				// if it is sync, remove
				if cp.forceFlagValues.Force {
					startTime := time.Now()
					removeErr := cp.filesystem.RemoveFile(targetPath, true)
					endTime := time.Now()

					reportOverwrite(startTime, endTime, removeErr)

					if removeErr != nil {
						return removeErr
					}

					// fallthrough to copy entries
				} else {
					// ask
					overwrite := terminal.InputYN(fmt.Sprintf("Overwriting a collection %q, but data object exists. Overwrite?", targetPath))
					if overwrite {
						startTime := time.Now()
						removeErr := cp.filesystem.RemoveFile(targetPath, true)
						endTime := time.Now()

						reportOverwrite(startTime, endTime, removeErr)

						if removeErr != nil {
							return removeErr
						}

						// fallthrough to copy entries
					} else {
						overwriteErr := types.NewNotDirError(targetPath)
						now := time.Now()

						reportOverwrite(now, now, overwriteErr, "declined")
						terminal.Printf("skip copying a collection %q to %q. The data object already exists!\n", sourceEntry.Path, targetPath)
						logger.Debug("skip copying a collection. The data object already exists!")
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

	// copy entries
	entries, err := cp.filesystem.List(sourceEntry.Path)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a directory %q", sourceEntry.Path)
	}

	for _, entry := range entries {
		newEntryPath := path.MakeIRODSTargetFilePath(cp.filesystem, entry.Path, targetPath)

		if entry.IsDir() {
			// dir
			err = cp.copyDir(entry, newEntryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = cp.copyFile(entry, newEntryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cp *CpCommand) deleteExtraFile(targetEntry *irodsclient_fs.Entry) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetEntry.Path,
	})

	defaultNotes := []string{"cp", "extra", "file"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetEntry.Path,
			Error:    err,
			Notes:    newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
	}

	cp.mutex.RLock()
	isExtra := false
	if _, ok := cp.updatedPathMap[targetEntry.Path]; !ok {
		isExtra = true
	}
	cp.mutex.RUnlock()

	if isExtra {
		// extra file
		logger.Debug("removing an extra data object")

		if cp.forceFlagValues.Force {
			cp.scheduleDeleteExtraFile(targetEntry)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra data object %q. Remove?", targetEntry.Path))
			if overwrite {
				cp.scheduleDeleteExtraFile(targetEntry)
				return nil
			} else {
				// do not remove
				reportSimple(nil, "declined")
				return nil
			}
		}
	}

	return nil
}

func (cp *CpCommand) deleteExtraDir(targetEntry *irodsclient_fs.Entry) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetEntry.Path,
	})

	defaultNotes := []string{"cp", "extra", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetEntry.Path,
			Error:    err,
			Notes:    newNotes,
		}

		cp.transferReportManager.AddFile(reportFile)
	}

	// delete the directory itself
	cp.mutex.RLock()
	isExtra := false
	if _, ok := cp.updatedPathMap[targetEntry.Path]; !ok {
		isExtra = true
	}
	cp.mutex.RUnlock()

	if isExtra {
		// extra dir
		logger.Debug("removing an extra collection")

		if cp.forceFlagValues.Force {
			cp.scheduleDeleteExtraDir(targetEntry)
			return nil
		} else {
			// ask
			overwrite := terminal.InputYN(fmt.Sprintf("Removing an extra directory %q. Remove?", targetEntry.Path))
			if overwrite {
				cp.scheduleDeleteExtraDir(targetEntry)
				return nil
			} else {
				// do not remove
				reportSimple(nil, "declined")
				return nil
			}
		}
	}

	// scan recursively
	entries, err := cp.filesystem.List(targetEntry.Path)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a collection %q", targetEntry.Path)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// dir
			err = cp.deleteExtraDir(entry)
			if err != nil {
				return err
			}
		} else {
			// file
			err = cp.deleteExtraDir(entry)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

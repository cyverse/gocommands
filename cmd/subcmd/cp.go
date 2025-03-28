package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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
	flag.SetChecksumFlags(cpCmd, true, true)
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

	parallelJobManager    *commons.ParallelJobManager
	transferReportManager *commons.TransferReportManager
	updatedPathMap        map[string]bool
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
		return nil, xerrors.Errorf("failed to copy multiple source collections without creating root directory")
	}

	return cp, nil
}

func (cp *CpCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CpCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(cp.command)
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
	if cp.retryFlagValues.RetryNumber > 0 && !cp.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(cp.retryFlagValues.RetryNumber, cp.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", cp.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	cp.account = commons.GetSessionConfig().ToIRODSAccount()
	cp.filesystem, err = commons.GetIRODSFSClient(cp.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer cp.filesystem.Release()

	// transfer report
	cp.transferReportManager, err = commons.NewTransferReportManager(cp.transferReportFlagValues.Report, cp.transferReportFlagValues.ReportPath, cp.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer cp.transferReportManager.Release()

	// parallel job manager
	cp.parallelJobManager = commons.NewParallelJobManager(cp.filesystem, cp.parallelTransferFlagValues.ThreadNumber, cp.progressFlagValues.ShowProgress, cp.progressFlagValues.ShowFullPath)
	cp.parallelJobManager.Start()

	// Expand wildcards
	if cp.wildcardSearchFlagValues.WildcardSearch {
		cp.sourcePaths, err = commons.ExpandWildcards(cp.filesystem, cp.account, cp.sourcePaths, true, true)
		if err != nil {
			return xerrors.Errorf("failed to expand wildcards:  %w", err)
		}
	}

	// run
	if len(cp.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = cp.ensureTargetIsDir(cp.targetPath)
		if err != nil {
			return err
		}
	}

	for _, sourcePath := range cp.sourcePaths {
		err = cp.copyOne(sourcePath, cp.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to copy %q to %q: %w", sourcePath, cp.targetPath, err)
		}
	}

	cp.parallelJobManager.DoneScheduling()
	err = cp.parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel job: %w", err)
	}

	// delete extra
	if cp.syncFlagValues.Delete {
		logger.Infof("deleting extra files and directories under %q", cp.targetPath)

		err = cp.deleteExtra(cp.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func (cp *CpCommand) ensureTargetIsDir(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := cp.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := cp.filesystem.Stat(targetPath)
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

func (cp *CpCommand) copyOne(sourcePath string, targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := cp.account.ClientZone
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := cp.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		// dir
		if !cp.recursiveFlagValues.Recursive {
			return xerrors.Errorf("cannot copy a collection, turn on 'recurse' option")
		}

		if !cp.noRootFlagValues.NoRoot {
			targetPath = commons.MakeTargetIRODSFilePath(cp.filesystem, sourcePath, targetPath)
		}

		return cp.copyDir(sourceEntry, targetPath)
	}

	// file
	targetPath = commons.MakeTargetIRODSFilePath(cp.filesystem, sourcePath, targetPath)
	return cp.copyFile(sourceEntry, targetPath)
}

func (cp *CpCommand) scheduleCopy(sourceEntry *irodsclient_fs.Entry, targetPath string, targetEntry *irodsclient_fs.Entry) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CpCommand",
		"function": "scheduleCopy",
	})

	copyTask := func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		job.Progress(0, 1, false)

		logger.Debugf("copying a data object %q to %q", sourceEntry.Path, targetPath)
		err := fs.CopyFileToFile(sourceEntry.Path, targetPath, true)
		if err != nil {
			job.Progress(-1, 1, true)
			return xerrors.Errorf("failed to copy %q to %q: %w", sourceEntry.Path, targetPath, err)
		}

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:                  commons.TransferMethodCopy,
			StartAt:                 now,
			EndAt:                   now,
			SourcePath:              sourceEntry.Path,
			SourceSize:              sourceEntry.Size,
			SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
			SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
			DestPath:                targetPath,

			Notes: []string{},
		}

		if targetEntry != nil {
			reportFile.DestSize = targetEntry.Size
			reportFile.DestChecksumAlgorithm = string(targetEntry.CheckSumAlgorithm)
			reportFile.DestChecksum = hex.EncodeToString(targetEntry.CheckSum)
		}

		cp.transferReportManager.AddFile(reportFile)

		logger.Debugf("copied a data object %q to %q", sourceEntry.Path, targetPath)
		job.Progress(1, 1, false)

		job.Done()
		return nil
	}

	err := cp.parallelJobManager.Schedule(sourceEntry.Path, copyTask, 1, progress.UnitsDefault)
	if err != nil {
		return xerrors.Errorf("failed to schedule copy %q to %q: %w", sourceEntry.Path, targetPath, err)
	}

	logger.Debugf("scheduled a data object copy %q to %q", sourceEntry.Path, targetPath)

	return nil
}

func (cp *CpCommand) copyFile(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CpCommand",
		"function": "copyFile",
	})

	commons.MarkIRODSPathMap(cp.updatedPathMap, targetPath)

	if cp.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodCopy,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				SourceSize: sourceEntry.Size,
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			cp.transferReportManager.AddFile(reportFile)

			commons.Printf("skip copying a file %q to %q. The file is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debugf("skip copying a file %q to %q. The file is hidden!", sourceEntry.Path, targetPath)
			return nil
		}
	}

	if cp.syncFlagValues.Age > 0 {
		// check age
		age := time.Since(sourceEntry.ModifyTime)
		maxAge := time.Duration(cp.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodCopy,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				SourceSize: sourceEntry.Size,
				DestPath:   targetPath,
				Notes:      []string{"age", "skip"},
			}

			cp.transferReportManager.AddFile(reportFile)

			commons.Printf("skip copying a file %q to %q. The file is too old (%s > %s)!\n", sourceEntry.Path, targetPath, age, maxAge)
			logger.Debugf("skip copying a file %q to %q. The file is too old (%s > %s)!", sourceEntry.Path, targetPath, age, maxAge)
			return nil
		}
	}

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			return cp.scheduleCopy(sourceEntry, targetPath, nil)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if cp.syncFlagValues.Sync {
			// if it is sync, remove
			if cp.forceFlagValues.Force {
				removeErr := cp.filesystem.RemoveDir(targetPath, true, true)

				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodDelete,
					StartAt:    now,
					EndAt:      now,
					SourcePath: targetPath,
					Error:      removeErr,
					Notes:      []string{"overwrite", "cp", "dir"},
				}

				cp.transferReportManager.AddFile(reportFile)

				if removeErr != nil {
					return removeErr
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("overwriting a file %q, but directory exists. Overwrite?", targetPath))
				if overwrite {
					removeErr := cp.filesystem.RemoveDir(targetPath, true, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "cp", "dir"},
					}

					cp.transferReportManager.AddFile(reportFile)

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

	if cp.differentialTransferFlagValues.DifferentialTransfer {
		if cp.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceEntry.Size {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:                  commons.TransferMethodCopy,
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

					Notes: []string{"differential", "no_hash", "same file size", "skip"},
				}

				cp.transferReportManager.AddFile(reportFile)

				commons.Printf("skip copying a file %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip copying a file %q to %q. The file already exists!", sourceEntry.Path, targetPath)
				return nil
			}
		} else {
			if targetEntry.Size == sourceEntry.Size {
				// compare hash
				if len(sourceEntry.CheckSum) > 0 && bytes.Equal(sourceEntry.CheckSum, targetEntry.CheckSum) {
					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:                  commons.TransferMethodCopy,
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
						Notes:                   []string{"differential", "same checksum", "skip"},
					}

					cp.transferReportManager.AddFile(reportFile)

					commons.Printf("skip copying a file %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
					logger.Debugf("skip copying a file %q to %q. The file with the same hash already exists!", sourceEntry.Path, targetPath)
					return nil
				}
			}
		}
	} else {
		if !cp.forceFlagValues.Force {
			// ask
			overwrite := commons.InputYN(fmt.Sprintf("file %q already exists. Overwrite?", targetPath))
			if !overwrite {
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:                  commons.TransferMethodCopy,
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
					Notes:                   []string{"no_overwrite", "skip"},
				}

				cp.transferReportManager.AddFile(reportFile)

				commons.Printf("skip copying a file %q to %q. The file already exists!\n", sourceEntry.Path, targetPath)
				logger.Debugf("skip copying a file %q to %q. The file already exists!", sourceEntry.Path, targetPath)
				return nil
			}
		}
	}

	// schedule
	return cp.scheduleCopy(sourceEntry, targetPath, targetEntry)
}

func (cp *CpCommand) copyDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CpCommand",
		"function": "copyDir",
	})

	commons.MarkIRODSPathMap(cp.updatedPathMap, targetPath)

	if cp.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceEntry.Name, ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodCopy,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				SourceSize: sourceEntry.Size,
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			cp.transferReportManager.AddFile(reportFile)

			commons.Printf("skip copying a dir %q to %q. The dir is hidden!\n", sourceEntry.Path, targetPath)
			logger.Debugf("skip copying a dir %q to %q. The dir is hidden!", sourceEntry.Path, targetPath)
			return nil
		}
	}

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			err = cp.filesystem.MakeDir(targetPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make a directory %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodCopy,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			cp.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			if cp.syncFlagValues.Sync {
				// if it is sync, remove
				if cp.forceFlagValues.Force {
					removeErr := cp.filesystem.RemoveFile(targetPath, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "cp"},
					}

					cp.transferReportManager.AddFile(reportFile)

					if removeErr != nil {
						return removeErr
					}
				} else {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						removeErr := cp.filesystem.RemoveFile(targetPath, true)

						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:     commons.TransferMethodDelete,
							StartAt:    now,
							EndAt:      now,
							SourcePath: targetPath,
							Error:      removeErr,
							Notes:      []string{"overwrite", "cp"},
						}

						cp.transferReportManager.AddFile(reportFile)

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

	// copy entries
	entries, err := cp.filesystem.List(sourceEntry.Path)
	if err != nil {
		return xerrors.Errorf("failed to list a directory %q: %w", sourceEntry.Path, err)
	}

	for _, entry := range entries {
		newEntryPath := commons.MakeTargetIRODSFilePath(cp.filesystem, entry.Path, targetPath)

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

func (cp *CpCommand) deleteExtra(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := cp.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	return cp.deleteExtraInternal(targetPath)
}

func (cp *CpCommand) deleteExtraInternal(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CpCommand",
		"function": "deleteExtraInternal",
	})

	targetEntry, err := cp.filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target is file
	if !targetEntry.IsDir() {
		if _, ok := cp.updatedPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra data object %q", targetPath)

			removeErr := cp.filesystem.RemoveFile(targetPath, true)

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodDelete,
				StartAt:    now,
				EndAt:      now,
				SourcePath: targetPath,
				Error:      removeErr,
				Notes:      []string{"extra", "cp"},
			}

			cp.transferReportManager.AddFile(reportFile)

			if removeErr != nil {
				return removeErr
			}
		}

		return nil
	}

	// target is dir
	if _, ok := cp.updatedPathMap[targetPath]; !ok {
		// extra dir
		logger.Debugf("removing an extra collection %q", targetPath)

		removeErr := cp.filesystem.RemoveDir(targetPath, true, true)

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:     commons.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: targetPath,
			Error:      removeErr,
			Notes:      []string{"extra", "cp", "dir"},
		}

		cp.transferReportManager.AddFile(reportFile)

		if removeErr != nil {
			return removeErr
		}
	} else {
		// non extra dir
		// scan recursively
		entries, err := cp.filesystem.List(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to list a directory %q: %w", targetPath, err)
		}

		for _, entry := range entries {
			newTargetPath := path.Join(targetPath, entry.Name)
			err = cp.deleteExtraInternal(newTargetPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bputCmd = &cobra.Command{
	Use:     "bput <local-file-or-dir>... <target-collection>",
	Aliases: []string{"bundle_put"},
	Short:   "Bundle-upload files or directories to iRODS",
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
	flag.SetPostTransferFlagValues(bputCmd)
	flag.SetHiddenFileFlags(bputCmd)
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
	postTransferFlagValues         *flag.PostTransferFlagValues
	hiddenFileFlagValues           *flag.HiddenFileFlagValues
	transferReportFlagValues       *flag.TransferReportFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string

	bundleTransferManager *commons.BundleTransferManager
	transferReportManager *commons.TransferReportManager
	updatedPathMap        map[string]bool
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
		postTransferFlagValues:         flag.GetPostTransferFlagValues(),
		hiddenFileFlagValues:           flag.GetHiddenFileFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	bput.maxConnectionNum = bput.parallelTransferFlagValues.ThreadNumber + 2 // 2 for extraction

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
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// clear local
	// delete local bundles before entering to retry
	if bput.bundleTransferFlagValues.ClearOld {
		commons.CleanUpOldLocalBundles(bput.bundleTransferFlagValues.LocalTempPath, true)
	}

	// handle retry
	if bput.retryFlagValues.RetryNumber > 0 && !bput.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(bput.retryFlagValues.RetryNumber, bput.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", bput.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// Create a file system
	bput.account = commons.GetSessionConfig().ToIRODSAccount()
	bput.filesystem, err = commons.GetIRODSFSClientForLargeFileIO(bput.account, bput.maxConnectionNum, bput.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bput.filesystem.Release()

	if bput.commonFlagValues.TimeoutUpdated {
		commons.UpdateIRODSFSClientTimeout(bput.filesystem, bput.commonFlagValues.Timeout)
	}

	// transfer report
	bput.transferReportManager, err = commons.NewTransferReportManager(bput.transferReportFlagValues.Report, bput.transferReportFlagValues.ReportPath, bput.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer bput.transferReportManager.Release()

	// run
	// target must be a dir
	err = bput.ensureTargetIsDir(bput.targetPath)
	if err != nil {
		return xerrors.Errorf("target path %q is not a directory: %w", bput.targetPath, err)
	}

	// get staging path
	stagingDirPath, err := bput.getStagingDir(bput.targetPath)
	if err != nil {
		return xerrors.Errorf("failed to get staging path for target path %q: %w", bput.targetPath, err)
	}

	// clear old irods bundles
	if bput.bundleTransferFlagValues.ClearOld {
		logger.Debugf("clearing an irods temp directory %q", stagingDirPath)
		err = commons.CleanUpOldIRODSBundles(bput.filesystem, stagingDirPath, false, true)
		if err != nil {
			return xerrors.Errorf("failed to clean up old irods bundle files in %q: %w", stagingDirPath, err)
		}
	}

	// bundle root path
	localBundleRootPath := string(filepath.Separator)
	localBundleRootPath, err = commons.GetCommonRootLocalDirPath(bput.sourcePaths)
	if err != nil {
		return xerrors.Errorf("failed to get a common root directory for source paths: %w", err)
	}

	if !bput.noRootFlagValues.NoRoot {
		// use parent dir
		localBundleRootPath = filepath.Dir(localBundleRootPath)
	}

	// bundle transfer manager
	bput.bundleTransferManager = commons.NewBundleTransferManager(bput.account, bput.filesystem, bput.transferReportManager, bput.targetPath, localBundleRootPath, bput.bundleTransferFlagValues.MinFileNum, bput.bundleTransferFlagValues.MaxFileNum, bput.bundleTransferFlagValues.MaxFileSize, bput.parallelTransferFlagValues.ThreadNumber, bput.parallelTransferFlagValues.ThreadNumberPerFile, bput.parallelTransferFlagValues.RedirectToResource, bput.parallelTransferFlagValues.Icat, bput.bundleTransferFlagValues.LocalTempPath, stagingDirPath, bput.bundleTransferFlagValues.NoBulkRegistration, bput.checksumFlagValues.VerifyChecksum, bput.progressFlagValues.ShowProgress, bput.progressFlagValues.ShowFullPath)
	err = bput.bundleTransferManager.Start()
	if err != nil {
		return xerrors.Errorf("failed to start bundle transfer manager: %w", err)
	}

	// run
	for _, sourcePath := range bput.sourcePaths {
		err = bput.bputOne(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to bundle-put %q to %q: %w", sourcePath, bput.targetPath, err)
		}
	}

	bput.bundleTransferManager.DoneScheduling()
	err = bput.bundleTransferManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to bundle-put: %w", err)
	}

	// delete on success
	if bput.postTransferFlagValues.DeleteOnSuccess {
		for _, sourcePath := range bput.sourcePaths {
			logger.Infof("deleting source %q after successful data put", sourcePath)

			err := bput.deleteOnSuccess(sourcePath)
			if err != nil {
				return xerrors.Errorf("failed to delete source %q: %w", sourcePath, err)
			}
		}
	}

	// delete extra
	if bput.syncFlagValues.Delete {
		logger.Infof("deleting extra files and directories under %q", bput.targetPath)

		err = bput.deleteExtra(bput.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func (bput *BputCommand) ensureTargetIsDir(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "ensureTargetIsDir",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := bput.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// not exist
			logger.Debugf("creating a target directory %q", targetPath)
			return bput.filesystem.MakeDir(targetPath, true)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		return commons.NewNotDirError(targetPath)
	}

	return nil
}

func (bput *BputCommand) getStagingDir(targetPath string) (string, error) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "getStagingDir",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := bput.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	if len(bput.bundleTransferFlagValues.IRODSTempPath) > 0 {
		stagingPath := commons.MakeIRODSPath(cwd, home, zone, bput.bundleTransferFlagValues.IRODSTempPath)

		createdDir := false
		tempEntry, err := bput.filesystem.Stat(stagingPath)
		if err != nil {
			if irodsclient_types.IsFileNotFoundError(err) {
				// not exist
				err = bput.filesystem.MakeDir(stagingPath, true)
				if err != nil {
					// failed to
					return "", xerrors.Errorf("failed to make a collection %q: %w", stagingPath, err)
				}
				createdDir = true
			} else {
				return "", xerrors.Errorf("failed to stat %q: %w", stagingPath, err)
			}
		}

		if !tempEntry.IsDir() {
			return "", xerrors.Errorf("staging path %q is a file", stagingPath)
		}

		// is it safe?
		logger.Debugf("validating staging directory %q", stagingPath)

		err = commons.IsSafeStagingDir(stagingPath)
		if err != nil {
			logger.Debugf("staging path %q is not safe", stagingPath)

			if createdDir {
				bput.filesystem.RemoveDir(stagingPath, true, true)
			}

			return "", xerrors.Errorf("staging path %q is not safe: %w", stagingPath, err)
		}

		ok, err := commons.IsSameResourceServer(bput.filesystem, targetPath, stagingPath)
		if err != nil {
			logger.WithError(err).Debugf("failed to validate staging directory %q and target %q", stagingPath, targetPath)

			if createdDir {
				bput.filesystem.RemoveDir(stagingPath, true, true)
			}

			stagingPath = commons.GetDefaultStagingDir(targetPath)
			logger.WithError(err).Debugf("use default staging path %q for target %q", stagingPath, targetPath)
			return stagingPath, nil
		}

		if !ok {
			logger.Debugf("staging directory %q is in a different resource server as target %q", stagingPath, targetPath)

			if createdDir {
				bput.filesystem.RemoveDir(stagingPath, true, true)
			}

			stagingPath = commons.GetDefaultStagingDir(targetPath)
			logger.Debugf("use default staging path %q for target %q", stagingPath, targetPath)
			return stagingPath, nil
		}

		logger.Debugf("use staging path %q for target %q", stagingPath, targetPath)
		return stagingPath, nil
	}

	// use default staging dir
	stagingPath := commons.GetDefaultStagingDir(targetPath)

	err := commons.IsSafeStagingDir(stagingPath)
	if err != nil {
		logger.Debugf("staging path %q is not safe", stagingPath)

		return "", xerrors.Errorf("staging path %q is not safe: %w", stagingPath, err)
	}

	// may not exist
	err = bput.filesystem.MakeDir(stagingPath, true)
	if err != nil {
		// failed to
		return "", xerrors.Errorf("failed to make a collection %q: %w", stagingPath, err)
	}

	logger.Debugf("use default staging path %q for target %q", stagingPath, targetPath)
	return stagingPath, nil
}

func (bput *BputCommand) bputOne(sourcePath string) error {
	sourcePath = commons.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceStat.IsDir() {
		// dir
		return bput.putDir(sourceStat, sourcePath)
	}

	// file
	return bput.putFile(sourceStat, sourcePath)
}

func (bput *BputCommand) schedulePut(sourceStat fs.FileInfo, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "schedulePut",
	})

	err := bput.bundleTransferManager.Schedule(sourceStat, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to schedule a file %q: %w", sourcePath, err)
	}

	logger.Debugf("scheduled a file upload %q", sourcePath)

	return nil
}

func (bput *BputCommand) putFile(sourceStat fs.FileInfo, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "putFile",
	})

	targetPath, err := bput.bundleTransferManager.GetTargetPath(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to get target path for source %q: %w", sourcePath, err)
	}

	commons.MarkIRODSPathMap(bput.updatedPathMap, targetPath)

	if bput.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodBput,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			bput.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a file %q to %q. The file is hidden!\n", sourcePath, targetPath)
			logger.Debugf("skip uploading a file %q to %q. The file is hidden!", sourcePath, targetPath)
			return nil
		}
	}

	if bput.syncFlagValues.Age > 0 {
		// check age
		age := time.Since(sourceStat.ModTime())
		maxAge := time.Duration(bput.syncFlagValues.Age) * time.Minute
		if age > maxAge {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodBput,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"age", "skip"},
			}

			bput.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a file %q to %q. The file is too old (%s > %s)!\n", sourcePath, targetPath, age, maxAge)
			logger.Debugf("skip uploading a file %q to %q. The file is too old (%s > %s)!", sourcePath, targetPath, age, maxAge)
			return nil
		}
	}

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			return bput.schedulePut(sourceStat, sourcePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		if bput.syncFlagValues.Sync {
			// if it is sync, remove
			if bput.forceFlagValues.Force {
				removeErr := bput.filesystem.RemoveDir(targetPath, true, true)

				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodDelete,
					StartAt:    now,
					EndAt:      now,
					SourcePath: targetPath,
					Error:      removeErr,
					Notes:      []string{"overwrite", "put", "dir"},
				}

				bput.transferReportManager.AddFile(reportFile)

				if removeErr != nil {
					return removeErr
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("overwriting a file %q, but directory exists. Overwrite?", targetPath))
				if overwrite {
					removeErr := bput.filesystem.RemoveDir(targetPath, true, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "put", "dir"},
					}

					bput.transferReportManager.AddFile(reportFile)

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

	if bput.differentialTransferFlagValues.DifferentialTransfer {
		if bput.differentialTransferFlagValues.NoHash {
			if targetEntry.Size == sourceStat.Size() {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:     commons.TransferMethodBput,
					StartAt:    now,
					EndAt:      now,
					SourcePath: sourcePath,
					SourceSize: sourceStat.Size(),

					DestPath:              targetEntry.Path,
					DestSize:              targetEntry.Size,
					DestChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),
					Notes:                 []string{"differential", "no_hash", "same file size", "skip"},
				}

				bput.transferReportManager.AddFile(reportFile)

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
						return xerrors.Errorf("failed to get hash %q: %w", sourcePath, err)
					}

					if bytes.Equal(localChecksum, targetEntry.CheckSum) {
						// skip
						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:                  commons.TransferMethodBput,
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

						bput.transferReportManager.AddFile(reportFile)

						commons.Printf("skip uploading a file %q to %q. The file with the same hash already exists!\n", sourcePath, targetPath)
						logger.Debugf("skip uploading a file %q to %q. The file with the same hash already exists!", sourcePath, targetPath)
						return nil
					}
				}
			}
		}
	} else {
		if !bput.forceFlagValues.Force {
			// ask
			overwrite := commons.InputYN(fmt.Sprintf("file %q already exists. Overwrite?", targetPath))
			if !overwrite {
				// skip
				now := time.Now()
				reportFile := &commons.TransferReportFile{
					Method:                commons.TransferMethodBput,
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

				bput.transferReportManager.AddFile(reportFile)

				commons.Printf("skip uploading a file %q to %q. The data object already exists!\n", sourcePath, targetPath)
				logger.Debugf("skip uploading a file %q to %q. The data object already exists!", sourcePath, targetPath)
				return nil
			}
		}
	}

	// schedule
	return bput.schedulePut(sourceStat, sourcePath)
}

func (bput *BputCommand) putDir(sourceStat fs.FileInfo, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "putDir",
	})

	targetPath, err := bput.bundleTransferManager.GetTargetPath(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to get target path for source %q: %w", sourcePath, err)
	}

	commons.MarkIRODSPathMap(bput.updatedPathMap, targetPath)

	if bput.hiddenFileFlagValues.Exclude {
		// exclude hidden
		if strings.HasPrefix(sourceStat.Name(), ".") {
			// skip
			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodBput,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				SourceSize: sourceStat.Size(),
				DestPath:   targetPath,
				Notes:      []string{"hidden", "skip"},
			}

			bput.transferReportManager.AddFile(reportFile)

			commons.Printf("skip uploading a dir %q to %q. The dir is hidden!\n", sourcePath, targetPath)
			logger.Debugf("skip uploading a dir %q to %q. The dir is hidden!", sourcePath, targetPath)
			return nil
		}
	}

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			err = bput.filesystem.MakeDir(targetPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make a collection %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodBput,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			bput.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			if bput.syncFlagValues.Sync {
				// if it is sync, remove
				if bput.forceFlagValues.Force {
					removeErr := bput.filesystem.RemoveFile(targetPath, true)

					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:     commons.TransferMethodDelete,
						StartAt:    now,
						EndAt:      now,
						SourcePath: targetPath,
						Error:      removeErr,
						Notes:      []string{"overwrite", "put"},
					}

					bput.transferReportManager.AddFile(reportFile)

					if removeErr != nil {
						return removeErr
					}
				} else {
					// ask
					overwrite := commons.InputYN(fmt.Sprintf("overwriting a directory %q, but file exists. Overwrite?", targetPath))
					if overwrite {
						removeErr := bput.filesystem.RemoveFile(targetPath, true)

						now := time.Now()
						reportFile := &commons.TransferReportFile{
							Method:     commons.TransferMethodDelete,
							StartAt:    now,
							EndAt:      now,
							SourcePath: targetPath,
							Error:      removeErr,
							Notes:      []string{"overwrite", "put"},
						}

						bput.transferReportManager.AddFile(reportFile)

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

	// get entries
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to read a directory %q: %w", sourcePath, err)
	}

	for _, entry := range entries {
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
			err = bput.putDir(entryStat, entryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = bput.putFile(entryStat, entryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

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

func (bput *BputCommand) deleteExtra(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := bput.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	return bput.deleteExtraInternal(targetPath)
}

func (bput *BputCommand) deleteExtraInternal(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BputCommand",
		"function": "deleteExtraInternal",
	})

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		// file
		if _, ok := bput.updatedPathMap[targetPath]; !ok {
			// extra file
			logger.Debugf("removing an extra data object %q", targetPath)

			removeErr := bput.filesystem.RemoveFile(targetPath, true)

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodDelete,
				StartAt:    now,
				EndAt:      now,
				SourcePath: targetPath,
				Error:      removeErr,
				Notes:      []string{"extra", "put"},
			}

			bput.transferReportManager.AddFile(reportFile)

			if removeErr != nil {
				return removeErr
			}
		}

		return nil
	}

	// target is dir
	if _, ok := bput.updatedPathMap[targetPath]; !ok {
		// extra dir
		logger.Debugf("removing an extra collection %q", targetPath)

		removeErr := bput.filesystem.RemoveDir(targetPath, true, true)

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:     commons.TransferMethodDelete,
			StartAt:    now,
			EndAt:      now,
			SourcePath: targetPath,
			Error:      removeErr,
			Notes:      []string{"extra", "put", "dir"},
		}

		bput.transferReportManager.AddFile(reportFile)

		if removeErr != nil {
			return removeErr
		}
	} else {
		// non extra dir
		// scan recursively
		entries, err := bput.filesystem.List(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to list a directory %q: %w", targetPath, err)
		}

		for _, entry := range entries {
			newTargetPath := path.Join(targetPath, entry.Name)
			err = bput.deleteExtraInternal(newTargetPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

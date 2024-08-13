package subcmd

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bputCmd = &cobra.Command{
	Use:     "bput [local file1] [local file2] [local dir1] ... [collection]",
	Aliases: []string{"bundle_put"},
	Short:   "Bundle-upload files or directories",
	Long:    `This uploads files or directories to the given iRODS collection. The files or directories are bundled with TAR to maximize data transfer bandwidth, then extracted in the iRODS.`,
	RunE:    processBputCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bputCmd, false)

	flag.SetBundleTempFlags(bputCmd)
	flag.SetBundleClearFlags(bputCmd)
	flag.SetBundleConfigFlags(bputCmd)
	flag.SetParallelTransferFlags(bputCmd, true)
	flag.SetForceFlags(bputCmd, true)
	flag.SetProgressFlags(bputCmd)
	flag.SetRetryFlags(bputCmd)
	flag.SetDifferentialTransferFlags(bputCmd, true)
	flag.SetNoRootFlags(bputCmd)
	flag.SetSyncFlags(bputCmd)
	flag.SetTransferReportFlags(putCmd)

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

	forceFlagValues                *flag.ForceFlagValues
	bundleTempFlagValues           *flag.BundleTempFlagValues
	bundleClearFlagValues          *flag.BundleClearFlagValues
	bundleConfigFlagValues         *flag.BundleConfigFlagValues
	parallelTransferFlagValues     *flag.ParallelTransferFlagValues
	progressFlagValues             *flag.ProgressFlagValues
	retryFlagValues                *flag.RetryFlagValues
	differentialTransferFlagValues *flag.DifferentialTransferFlagValues
	checksumFlagValues             *flag.ChecksumFlagValues
	noRootFlagValues               *flag.NoRootFlagValues
	syncFlagValues                 *flag.SyncFlagValues
	postTransferFlagValues         *flag.PostTransferFlagValues
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

		forceFlagValues:                flag.GetForceFlagValues(),
		bundleTempFlagValues:           flag.GetBundleTempFlagValues(),
		bundleClearFlagValues:          flag.GetBundleClearFlagValues(),
		bundleConfigFlagValues:         flag.GetBundleConfigFlagValues(),
		parallelTransferFlagValues:     flag.GetParallelTransferFlagValues(),
		progressFlagValues:             flag.GetProgressFlagValues(),
		retryFlagValues:                flag.GetRetryFlagValues(),
		differentialTransferFlagValues: flag.GetDifferentialTransferFlagValues(),
		checksumFlagValues:             flag.GetChecksumFlagValues(),
		noRootFlagValues:               flag.GetNoRootFlagValues(),
		syncFlagValues:                 flag.GetSyncFlagValues(),
		postTransferFlagValues:         flag.GetPostTransferFlagValues(),
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	bput.maxConnectionNum = bput.parallelTransferFlagValues.ThreadNumber + 2 + 2 // 2 for metadata op, 2 for extraction

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
	if bput.bundleClearFlagValues.Clear {
		commons.CleanUpOldLocalBundles(bput.bundleTempFlagValues.LocalTempPath, true)
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
	bput.account = commons.GetAccount()
	bput.filesystem, err = commons.GetIRODSFSClientAdvanced(bput.account, bput.maxConnectionNum, bput.parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bput.filesystem.Release()

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
		return err
	}

	// get staging path
	stagingDirPath, err := bput.getStagingDir(bput.targetPath)
	if err != nil {
		return err
	}

	// clear old irods bundles
	if bput.bundleClearFlagValues.Clear {
		logger.Debugf("clearing an irods temp directory %q", stagingDirPath)
		err = commons.CleanUpOldIRODSBundles(bput.filesystem, stagingDirPath, false, true)
		if err != nil {
			return xerrors.Errorf("failed to clean up old irods bundle files in %q: %w", stagingDirPath, err)
		}
	}

	// bundle root path
	bundleRootPath := "/"
	bundleRootPath, err = commons.GetCommonRootLocalDirPath(bput.sourcePaths)
	if err != nil {
		return xerrors.Errorf("failed to get a common root directory for source paths: %w", err)
	}

	if !bput.noRootFlagValues.NoRoot {
		// use parent dir
		bundleRootPath = filepath.Dir(bundleRootPath)
	}

	// bundle transfer manager
	bput.bundleTransferManager = commons.NewBundleTransferManager(bput.filesystem, bput.targetPath, bundleRootPath, bput.bundleConfigFlagValues.MaxFileNum, bput.bundleConfigFlagValues.MaxFileSize, bput.parallelTransferFlagValues.SingleTread, bput.parallelTransferFlagValues.ThreadNumber, bput.parallelTransferFlagValues.RedirectToResource, bput.parallelTransferFlagValues.Icat, bput.bundleTempFlagValues.LocalTempPath, bput.bundleTempFlagValues.IRODSTempPath, bput.differentialTransferFlagValues.DifferentialTransfer, bput.differentialTransferFlagValues.NoHash, bput.bundleConfigFlagValues.NoBulkRegistration, bput.progressFlagValues.ShowProgress, bput.progressFlagValues.ShowFullPath)
	bput.bundleTransferManager.Start()

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
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := bput.filesystem.Stat(targetPath)
	if err != nil {
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
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	if len(bput.bundleTempFlagValues.IRODSTempPath) > 0 {
		stagingPath := commons.MakeIRODSPath(cwd, home, zone, bput.bundleTempFlagValues.IRODSTempPath)

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
			logger.Debugf("failed to validate staging directory %q and target %q - %s", stagingPath, targetPath, err.Error())

			if createdDir {
				bput.filesystem.RemoveDir(stagingPath, true, true)
			}

			stagingPath = commons.GetDefaultStagingDir(targetPath)
			logger.Debugf("use default staging path %q for target %q - %s", stagingPath, targetPath, err.Error())
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

func (bput *BputCommand) putFile(sourceStat fs.FileInfo, sourcePath string) error {
	err := bput.bundleTransferManager.Schedule(sourcePath, false, sourceStat.Size(), sourceStat.ModTime().Local())
	if err != nil {
		return xerrors.Errorf("failed to schedule a file %q: %w", sourcePath, err)
	}

	//commons.MarkPathMap(bput.updatedPathMap, targetPath)

	return nil
}

func (bput *BputCommand) putDir(sourceStat fs.FileInfo, sourcePath string) error {
	err := bput.bundleTransferManager.Schedule(sourcePath, true, 0, sourceStat.ModTime().Local())
	if err != nil {
		return xerrors.Errorf("failed to schedule a directory %q: %w", sourcePath, err)
	}

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

			//commons.MarkPathMap(bput.updatedPathMap, newEntryPath)
		} else {
			// file
			err = bput.putFile(entryStat, entryPath)
			if err != nil {
				return err
			}

			//commons.MarkPathMap(bput.updatedPathMap, newEntryPath)
		}
	}

	//commons.MarkPathMap(put.updatedPathMap, targetPath)

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
	zone := commons.GetZone()
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

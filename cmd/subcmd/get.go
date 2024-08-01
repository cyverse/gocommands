package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
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
	flag.SetTicketAccessFlags(getCmd)
	flag.SetParallelTransferFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetDifferentialTransferFlags(getCmd, true)
	flag.SetChecksumFlags(getCmd, false)
	flag.SetTransferReportFlags(getCmd)
	flag.SetNoRootFlags(getCmd)
	flag.SetSyncFlags(getCmd)
	flag.SetDecryptionFlags(getCmd)
	flag.SetPostTransferFlagValues(getCmd)

	rootCmd.AddCommand(getCmd)
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
	transferReportFlagValues       *flag.TransferReportFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string

	parallelJobManager    *commons.ParallelJobManager
	transferReportManager *commons.TransferReportManager
	inputPathMap          map[string]bool
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
		transferReportFlagValues:       flag.GetTransferReportFlagValues(command),

		inputPathMap: map[string]bool{},
	}

	get.maxConnectionNum = get.parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	// path
	get.targetPath = "./"
	get.sourcePaths = args[:]

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
		logger.Debugf("use ticket: %s", get.ticketAccessFlagValues.Name)
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

	if get.noRootFlagValues.NoRoot && len(get.sourcePaths) > 1 {
		return xerrors.Errorf("failed to get multiple source collections without creating root directory")
	}

	// parallel job manager
	get.parallelJobManager = commons.NewParallelJobManager(get.filesystem, get.parallelTransferFlagValues.ThreadNumber, get.progressFlagValues.ShowProgress, get.progressFlagValues.ShowFullPath)
	get.parallelJobManager.Start()

	// run
	for _, sourcePath := range get.sourcePaths {
		newSourcePath, newTargetDirPath, err := get.makeSourceAndTargetDirPath(sourcePath, get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to make new target path for get %s to %s: %w", sourcePath, get.targetPath, err)
		}

		err = get.getOne(newSourcePath, newTargetDirPath, get.decryptionFlagValues.Decryption)
		if err != nil {
			return xerrors.Errorf("failed to perform get %s to %s: %w", newSourcePath, newTargetDirPath, err)
		}
	}

	get.parallelJobManager.DoneScheduling()
	err = get.parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	return nil
}

func (get *GetCommand) makeSourceAndTargetDirPath(sourcePath string, targetPath string) (string, string, error) {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return "", "", xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := os.Stat(targetDirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", "", irodsclient_types.NewFileNotFoundError(targetDirPath)
			}

			return "", "", xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		return sourcePath, targetDirPath, nil
	}

	// dir
	_, err = os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", irodsclient_types.NewFileNotFoundError(targetPath)
		}

		return "", "", xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
	}

	targetDirPath := targetPath

	if !get.noRootFlagValues.NoRoot {
		// make target dir
		targetDirPath = commons.MakeTargetLocalFilePath(sourceEntry.Path, targetDirPath)
		err = os.MkdirAll(targetDirPath, 0766)
		if err != nil {
			return "", "", xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
		}
	}

	return sourcePath, targetDirPath, nil
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

func (get *GetCommand) getOne(sourcePath string, targetPath string, requireDecryption bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "getOne",
	})

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	// load encryption config
	if !requireDecryption {
		requireDecryption = get.requireDecryptionByMeta(sourceEntry)
	}

	// if source is dir, recursive
	if sourceEntry.Type == irodsclient_fs.DirectoryEntry {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		for _, entry := range entries {
			targetDirPath := targetPath
			if entry.Type != irodsclient_fs.FileEntry {
				// dir
				targetDirPath = commons.MakeTargetLocalFilePath(entry.Path, targetPath)
				err = os.MkdirAll(targetDirPath, 0766)
				if err != nil {
					return xerrors.Errorf("failed to make dir %s: %w", targetDirPath, err)
				}
			}

			commons.MarkPathMap(get.inputPathMap, targetDirPath)

			err = get.getOne(entry.Path, targetDirPath, requireDecryption)
			if err != nil {
				return xerrors.Errorf("failed to perform get %s to %s: %w", entry.Path, targetDirPath, err)
			}
		}

		return nil
	}

	if sourceEntry.Type != irodsclient_fs.FileEntry {
		return xerrors.Errorf("unhandled file entry type %s", sourceEntry.Type)
	}

	// file

	commons.MarkPathMap(get.inputPathMap, decryptedTargetFilePath)

	fileExist := false
	targetEntry, err := os.Stat(targetFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
		}
	} else {
		fileExist = true
	}

	getTask := get.getTask(sourcePath, targetFilePath, sourceEntry.Size)

	if fileExist {
		// check transfer status file
		trxStatusFilePath := irodsclient_irodsfs.GetDataObjectTransferStatusFilePath(targetFilePath)
		trxStatusFileExist := false
		_, err = os.Stat(trxStatusFilePath)
		if err == nil {
			trxStatusFileExist = true
		}

		if trxStatusFileExist {
			// incomplete file - resume downloading
			commons.Printf("resume downloading a data object %s\n", targetFilePath)
			logger.Debugf("resume downloading a data object %s", targetFilePath)
		} else if get.differentialTransferFlagValues.DifferentialTransfer {
			// trx status not exist
			if get.differentialTransferFlagValues.NoHash {
				if targetEntry.Size() == sourceEntry.Size {
					// skip
					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:            commons.TransferMethodGet,
						StartAt:           now,
						EndAt:             now,
						LocalPath:         targetFilePath,
						LocalSize:         targetEntry.Size(),
						IrodsPath:         sourcePath,
						IrodsSize:         sourceEntry.Size,
						IrodsChecksum:     hex.EncodeToString(sourceEntry.CheckSum),
						ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
						Notes:             []string{"differential", "no_hash", "same file size", "skip"},
					}

					get.transferReportManager.AddFile(reportFile)

					commons.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
					logger.Debugf("skip downloading a data object %s. The file already exists!", targetFilePath)
					return nil
				}
			} else {
				if targetEntry.Size() == sourceEntry.Size {
					if len(sourceEntry.CheckSum) > 0 {
						// compare hash
						hash, err := irodsclient_util.HashLocalFile(targetFilePath, string(sourceEntry.CheckSumAlgorithm))
						if err != nil {
							return xerrors.Errorf("failed to get hash of %s: %w", targetFilePath, err)
						}

						if bytes.Equal(sourceEntry.CheckSum, hash) {
							// skip
							now := time.Now()
							reportFile := &commons.TransferReportFile{
								Method:            commons.TransferMethodGet,
								StartAt:           now,
								EndAt:             now,
								LocalPath:         targetFilePath,
								LocalSize:         targetEntry.Size(),
								IrodsPath:         sourcePath,
								IrodsSize:         sourceEntry.Size,
								IrodsChecksum:     hex.EncodeToString(sourceEntry.CheckSum),
								ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
								Notes:             []string{"differential", "same hash", "same file size", "skip"},
							}

							get.transferReportManager.AddFile(reportFile)

							commons.Printf("skip downloading a data object %s. The file with the same hash already exists!\n", targetFilePath)
							logger.Debugf("skip downloading a data object %s. The file with the same hash already exists!", targetFilePath)
							return nil
						}
					}
				}
			}
		} else {
			if !get.forceFlagValues.Force {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if !overwrite {
					// skip
					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:            commons.TransferMethodGet,
						StartAt:           now,
						EndAt:             now,
						LocalPath:         targetFilePath,
						LocalSize:         targetEntry.Size(),
						IrodsPath:         sourcePath,
						IrodsSize:         sourceEntry.Size,
						IrodsChecksum:     hex.EncodeToString(sourceEntry.CheckSum),
						ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
						Notes:             []string{"no overwrite", "skip"},
					}

					get.transferReportManager.AddFile(reportFile)

					commons.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
					logger.Debugf("skip downloading a data object %s. The file already exists!", targetFilePath)
					return nil
				}
			}
		}
	}

	threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
	err = get.parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
	if err != nil {
		return xerrors.Errorf("failed to schedule %s: %w", sourcePath, err)
	}

	logger.Debugf("scheduled a data object download %s to %s", sourcePath, targetPath)
	return nil
}

func (get *GetCommand) requireDecryptionByMeta(sourceEntry *irodsclient_fs.Entry) bool {
	// load encryption config from meta
	if !get.decryptionFlagValues.NoDecryption && !get.decryptionFlagValues.IgnoreMeta {
		sourceDir := sourceEntry.Path
		if !sourceEntry.IsDir() {
			sourceDir = commons.GetDir(sourceEntry.Path)
		}

		encryptionConfig := commons.GetEncryptionConfigFromMeta(get.filesystem, sourceDir)

		if encryptionConfig.Required {
			return encryptionConfig.Required
		}
	}

	return false
}

func (get *GetCommand) requireDecryption(sourcePath string) bool {
	encryptionMode := commons.DetectEncryptionMode(sourcePath)
	if get.decryptionFlagValues.Decryption && encryptionMode != commons.EncryptionModeUnknown {
		return true
	}

	return false
}

func (get *GetCommand) getPathsForDecryption(sourcePath string, targetPath string) (string, string, error) {
	sourceFilename := commons.GetBasename(sourcePath)

	encryptionMode := commons.DetectEncryptionMode(sourceFilename)
	encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

	if get.requireDecryption(sourcePath) {
		tempFilePath := commons.MakeTargetLocalFilePath(sourcePath, get.decryptionFlagValues.TempPath)

		decryptedFilename, err := encryptManager.DecryptFilename(sourceFilename)
		if err != nil {
			return "", "", xerrors.Errorf("failed to decrypt filename %s: %w", sourcePath, err)
		}

		targetFilePath := commons.MakeTargetLocalFilePath(decryptedFilename, targetPath)

		return tempFilePath, targetFilePath, nil
	}

	targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)

	return "", targetFilePath, nil
}

func (get *GetCommand) decryptFile(sourcePath string, encryptedFilePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "decryptFile",
	})

	encryptionMode := commons.DetectEncryptionMode(sourcePath)
	encryptManager := get.getEncryptionManagerForDecryption(encryptionMode)

	if get.requireDecryption(sourcePath) {
		logger.Debugf("decrypt a data object %s to %s", encryptedFilePath, targetPath)
		err := encryptManager.DecryptFile(encryptedFilePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to decrypt %s: %w", encryptedFilePath, err)
		}

		logger.Debugf("removing a temp file %s", encryptedFilePath)
		os.Remove(encryptedFilePath)
	}

	return nil
}

func (get *GetCommand) getTask(sourcePath string, targetPath string, sourceSize int64) func(job *commons.ParallelJob) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "getTask",
	})

	return func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		callbackGet := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, sourceSize, false)

		logger.Debugf("downloading a data object %s to %s", sourcePath, targetPath)

		var downloadErr error
		var downloadResult *irodsclient_fs.FileTransferResult
		notes := []string{}

		// determine how to download
		if get.parallelTransferFlagValues.SingleTread || get.parallelTransferFlagValues.ThreadNumber == 1 {
			downloadResult, downloadErr = fs.DownloadFileResumable(sourcePath, "", targetPath, get.checksumFlagValues.VerifyChecksum, callbackGet)
			notes = append(notes, "icat")
			notes = append(notes, "single-thread")
		} else if get.parallelTransferFlagValues.RedirectToResource {
			downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourcePath, "", targetPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
			notes = append(notes, "redirect-to-resource")
		} else if get.parallelTransferFlagValues.Icat {
			downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourcePath, "", targetPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
			notes = append(notes, "icat")
			notes = append(notes, "multi-thread")
		} else {
			// auto
			if sourceSize >= commons.RedirectToResourceMinSize {
				// redirect-to-resource
				downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourcePath, "", targetPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
				notes = append(notes, "redirect-to-resource")
			} else {
				downloadResult, downloadErr = fs.DownloadFileParallelResumable(sourcePath, "", targetPath, 0, get.checksumFlagValues.VerifyChecksum, callbackGet)
				notes = append(notes, "icat")
				notes = append(notes, "multi-thread")
			}
		}

		get.transferReportManager.AddTransfer(downloadResult, commons.TransferMethodGet, downloadErr, notes)

		if downloadErr != nil {
			job.Progress(-1, sourceSize, true)
			return xerrors.Errorf("failed to download %s to %s: %w", sourcePath, targetPath, downloadErr)
		}

		logger.Debugf("downloaded a data object %s to %s", sourcePath, targetPath)
		job.Progress(sourceSize, sourceSize, false)

		return nil
	}
}

func (get *GetCommand) deleteOnSuccess(sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "deleteOnSuccess",
	})

	if get.postTransferFlagValues.DeleteOnSuccess {
		logger.Debugf("removing source file %s", sourcePath)
		get.filesystem.RemoveFile(sourcePath, true)
	}

	return nil
}

func (get *GetCommand) deleteExtra() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "deleteExtra",
	})

	if get.syncFlagValues.Delete {
		logger.Infof("deleting extra files and dirs under %s", get.targetPath)
		targetPath := commons.MakeLocalPath(get.targetPath)

		err := get.deleteExtraInternal(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to delete extra files: %w", err)
		}
	}

	return nil
}

func (get *GetCommand) deleteExtraInternal(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "deleteExtraInternal",
	})

	realTargetPath, err := commons.ResolveSymlink(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to resolve symlink %s: %w", targetPath, err)
	}

	targetStat, err := os.Stat(realTargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(realTargetPath)
		}

		return xerrors.Errorf("failed to stat %s: %w", realTargetPath, err)
	}

	// if target is dir, recursive
	if targetStat.IsDir() {
		// dir
		if _, ok := get.inputPathMap[targetPath]; !ok {
			// extra dir
			logger.Debugf("removing an extra dir %s", targetPath)
			removeErr := os.RemoveAll(targetPath)

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:    commons.TransferMethodGet,
				StartAt:   now,
				EndAt:     now,
				LocalPath: targetPath,
				LocalSize: targetStat.Size(),
				Error:     removeErr,
				Notes:     []string{"deleted", "extra"},
			}

			get.transferReportManager.AddFile(reportFile)

			if removeErr != nil {
				return removeErr
			}
		} else {
			// non extra dir
			entries, err := os.ReadDir(targetPath)
			if err != nil {
				return xerrors.Errorf("failed to list dir %s: %w", targetPath, err)
			}

			for _, entryInDir := range entries {
				newTargetPath := commons.MakeTargetLocalFilePath(entryInDir.Name(), targetPath)
				err = get.deleteExtraInternal(newTargetPath)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	// file
	if _, ok := get.inputPathMap[targetPath]; !ok {
		// extra file
		logger.Debugf("removing an extra file %s", targetPath)
		removeErr := os.Remove(targetPath)

		now := time.Now()
		reportFile := &commons.TransferReportFile{
			Method:    commons.TransferMethodGet,
			StartAt:   now,
			EndAt:     now,
			LocalPath: targetPath,
			LocalSize: targetStat.Size(),
			Error:     removeErr,
			Notes:     []string{"deleted", "extra"},
		}

		get.transferReportManager.AddFile(reportFile)

		if removeErr != nil {
			return removeErr
		}
	}

	return nil
}

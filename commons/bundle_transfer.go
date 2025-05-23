package commons

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

// default values
const (
	MaxBundleFileNumDefault  int   = 50
	MaxBundleFileSizeDefault int64 = 2 * 1024 * 1024 * 1024 // 2GB
	MinBundleFileNumDefault  int   = 3
)

const (
	BundleTaskNameRemoveFilesAndMakeDirs string = "Preparing target"
	BundleTaskNameTar                    string = "Bundling"
	BundleTaskNameUpload                 string = "Uploading"
	BundleTaskNameExtract                string = "Extracting"
)

type BundleEntry struct {
	LocalPath string
	IRODSPath string
	Size      int64
	Dir       bool
}

type Bundle struct {
	manager *BundleTransferManager

	Index             int64
	Entries           []*BundleEntry
	Size              int64
	LocalBundlePath   string
	IRODSBundlePath   string
	LastError         error
	LastErrorTaskName string

	CreatedTarball   bool
	Uploaded         bool
	MadeDir          bool
	ExtractedTarball bool
}

func newBundle(manager *BundleTransferManager) (*Bundle, error) {
	bundle := &Bundle{
		manager:           manager,
		Index:             manager.getNextBundleIndex(),
		Entries:           []*BundleEntry{},
		Size:              0,
		LocalBundlePath:   "",
		IRODSBundlePath:   "",
		LastError:         nil,
		LastErrorTaskName: "",

		CreatedTarball:   false,
		Uploaded:         false,
		MadeDir:          false,
		ExtractedTarball: false,
	}

	err := bundle.updateBundlePath()
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (bundle *Bundle) GetManager() *BundleTransferManager {
	return bundle.manager
}

func (bundle *Bundle) GetEntries() []*BundleEntry {
	return bundle.Entries
}

func (bundle *Bundle) GetBundleFilename() (string, error) {
	entryStrs := []string{}

	entryStrs = append(entryStrs, "empty_bundle")

	for _, entry := range bundle.Entries {
		entryStrs = append(entryStrs, entry.LocalPath)
	}

	hash, err := irodsclient_util.HashStrings(entryStrs, string(irodsclient_types.ChecksumAlgorithmMD5))
	if err != nil {
		return "", err
	}

	hexhash := hex.EncodeToString(hash)

	return GetBundleFilename(hexhash), nil
}

func (bundle *Bundle) Add(sourceStat fs.FileInfo, sourcePath string) error {
	irodsPath, err := bundle.manager.GetTargetPath(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to get target path for %q: %w", sourcePath, err)
	}

	e := &BundleEntry{
		LocalPath: sourcePath,
		IRODSPath: irodsPath,
		Size:      sourceStat.Size(),
		Dir:       sourceStat.IsDir(),
	}

	bundle.Entries = append(bundle.Entries, e)
	if !sourceStat.IsDir() {
		bundle.Size += sourceStat.Size()
	}

	err = bundle.updateBundlePath()
	if err != nil {
		return err
	}

	return nil
}

func (bundle *Bundle) updateBundlePath() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "updateBundlePath",
	})

	filename, err := bundle.GetBundleFilename()
	if err != nil {
		return xerrors.Errorf("failed to get bundle filename: %w", err)
	}

	logger.Debugf("bundle local temp path %q, irods temp path %q", bundle.manager.localTempDirPath, bundle.manager.irodsTempDirPath)

	bundle.LocalBundlePath = filepath.Join(bundle.manager.localTempDirPath, filename)
	bundle.IRODSBundlePath = path.Join(bundle.manager.irodsTempDirPath, filename)

	logger.Debugf("bundle local path %q, irods path %q", bundle.LocalBundlePath, bundle.IRODSBundlePath)

	return nil
}

func (bundle *Bundle) isFull() bool {
	return bundle.Size >= bundle.manager.maxBundleFileSize || len(bundle.Entries) >= bundle.manager.maxBundleFileNum
}

func (bundle *Bundle) RequireTar() bool {
	return len(bundle.Entries) >= bundle.manager.minBundleFileNum
}

func (bundle *Bundle) GetThreadsRequired() int {
	return bundle.manager.calculateThreadForBundleTransfer(bundle)
}

type BundleTransferManager struct {
	// moved to top to avoid 64bit alignment issue
	bundlesScheduledCounter int64
	bundlesDoneCounter      int64

	account                      *irodsclient_types.IRODSAccount
	filesystem                   *irodsclient_fs.FileSystem
	transferReportManager        *TransferReportManager
	irodsDestPath                string
	currentBundle                *Bundle
	nextBundleIndex              int64
	pendingBundles               chan *Bundle
	bundles                      []*Bundle
	localBundleRootPath          string
	minBundleFileNum             int
	maxBundleFileNum             int
	maxBundleFileSize            int64
	maxTotalUploadThreads        int
	maxUploadThreadsPerFile      int
	redirectToResource           bool
	icat                         bool
	localTempDirPath             string
	irodsTempDirPath             string
	noBulkRegistration           bool
	verifyChecksum               bool
	showProgress                 bool
	showFullPath                 bool
	progressWriter               progress.Writer
	progressTrackers             map[string]*progress.Tracker
	progressTrackerCallback      ProgressTrackerCallback
	lastError                    error
	mutex                        sync.RWMutex
	availableThreadWaitCondition *sync.Cond // used for checking available threads

	scheduleWait sync.WaitGroup
	processWait  sync.WaitGroup
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(account *irodsclient_types.IRODSAccount, fs *irodsclient_fs.FileSystem, transferReportManager *TransferReportManager, irodsDestPath string, localBundleRootPath string, minBundleFileNum int, maxBundleFileNum int, maxBundleFileSize int64, maxTotalUploadThreads int, maxUploadThreadsPerFile int, redirectToResource bool, useIcat bool, localTempDirPath string, irodsTempDirPath string, noBulkReg bool, verifyChecksum bool, showProgress bool, showFullPath bool) *BundleTransferManager {
	cwd := GetCWD()
	home := GetHomeDir()
	zone := account.ClientZone
	irodsDestPath = MakeIRODSPath(cwd, home, zone, irodsDestPath)

	manager := &BundleTransferManager{
		account:                 account,
		filesystem:              fs,
		transferReportManager:   transferReportManager,
		irodsDestPath:           irodsDestPath,
		currentBundle:           nil,
		nextBundleIndex:         0,
		pendingBundles:          make(chan *Bundle, 100),
		bundles:                 []*Bundle{},
		localBundleRootPath:     localBundleRootPath,
		minBundleFileNum:        minBundleFileNum,
		maxBundleFileNum:        maxBundleFileNum,
		maxBundleFileSize:       maxBundleFileSize,
		maxTotalUploadThreads:   maxTotalUploadThreads,
		maxUploadThreadsPerFile: maxUploadThreadsPerFile,
		redirectToResource:      redirectToResource,
		icat:                    useIcat,
		localTempDirPath:        localTempDirPath,
		irodsTempDirPath:        irodsTempDirPath,
		noBulkRegistration:      noBulkReg,
		verifyChecksum:          verifyChecksum,
		showProgress:            showProgress,
		showFullPath:            showFullPath,
		progressWriter:          nil,
		progressTrackers:        map[string]*progress.Tracker{},
		progressTrackerCallback: nil,
		lastError:               nil,
		mutex:                   sync.RWMutex{},
		scheduleWait:            sync.WaitGroup{},
		processWait:             sync.WaitGroup{},

		bundlesScheduledCounter: 0,
		bundlesDoneCounter:      0,
	}

	manager.availableThreadWaitCondition = sync.NewCond(&manager.mutex)

	if manager.maxBundleFileNum <= 0 {
		manager.maxBundleFileNum = MaxBundleFileNumDefault
	}

	if manager.minBundleFileNum <= 0 {
		manager.minBundleFileNum = MinBundleFileNumDefault
	}

	manager.scheduleWait.Add(1)

	return manager
}

func (manager *BundleTransferManager) GetFilesystem() *irodsclient_fs.FileSystem {
	return manager.filesystem
}

func (manager *BundleTransferManager) getNextBundleIndex() int64 {
	idx := manager.nextBundleIndex
	manager.nextBundleIndex++
	return idx
}

func (manager *BundleTransferManager) progress(name string, processed int64, total int64, progressUnit progress.Units, errored bool) {
	if manager.progressTrackerCallback != nil {
		manager.progressTrackerCallback(name, processed, total, progressUnit, errored)
	}
}

func (manager *BundleTransferManager) GetTargetPath(localPath string) (string, error) {
	relPath, err := filepath.Rel(manager.localBundleRootPath, localPath)
	if err != nil {
		return "", xerrors.Errorf("failed to compute relative path %q to %q: %w", localPath, manager.localBundleRootPath, err)
	}

	return path.Join(manager.irodsDestPath, filepath.ToSlash(relPath)), nil
}

func (manager *BundleTransferManager) Schedule(sourceStat fs.FileInfo, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "Schedule",
	})

	manager.mutex.Lock()

	// do not accept new schedule if there's an error
	if manager.lastError != nil {
		defer manager.mutex.Unlock()
		return manager.lastError
	}

	if manager.currentBundle != nil {
		// if current bundle is full, prepare a new bundle
		if manager.currentBundle.isFull() {
			// temporarily release lock since adding to chan may block
			manager.mutex.Unlock()

			manager.pendingBundles <- manager.currentBundle
			manager.bundles = append(manager.bundles, manager.currentBundle)

			manager.mutex.Lock()
			manager.currentBundle = nil
			atomic.AddInt64(&manager.bundlesScheduledCounter, 1)
		}
	}

	if manager.currentBundle == nil {
		// add new
		bundle, err := newBundle(manager)
		if err != nil {
			return xerrors.Errorf("failed to create a new bundle for %q: %w", sourcePath, err)
		}

		manager.currentBundle = bundle
		logger.Debugf("assigned a new bundle %d", manager.currentBundle.Index)
	}

	defer manager.mutex.Unlock()

	logger.Debugf("scheduling a local file/directory bundle-upload %q", sourcePath)
	return manager.currentBundle.Add(sourceStat, sourcePath)
}

func (manager *BundleTransferManager) DoneScheduling() {
	manager.mutex.Lock()
	if manager.currentBundle != nil {
		manager.pendingBundles <- manager.currentBundle
		manager.bundles = append(manager.bundles, manager.currentBundle)
		manager.currentBundle = nil
		atomic.AddInt64(&manager.bundlesScheduledCounter, 1)
	}
	manager.mutex.Unlock()

	close(manager.pendingBundles)
	manager.scheduleWait.Done()
}

func (manager *BundleTransferManager) GetBundles() []*Bundle {
	return manager.bundles
}

func (manager *BundleTransferManager) Wait() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "Wait",
	})

	logger.Debug("waiting schedule-wait")
	manager.scheduleWait.Wait()
	logger.Debug("waiting transfer-wait")
	manager.processWait.Wait()

	manager.CleanUpBundles()

	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	if manager.lastError != nil {
		return manager.lastError
	}

	if manager.bundlesDoneCounter != manager.bundlesScheduledCounter {
		return xerrors.Errorf("%d bundles were done out of %d! Some bundles failed!", manager.bundlesDoneCounter, manager.bundlesScheduledCounter)
	}

	return nil
}

func (manager *BundleTransferManager) CleanUpBundles() {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "CleanUpBundles",
	})

	logger.Debugf("clearing bundle files in %q", manager.irodsTempDirPath)

	err := CleanUpOldIRODSBundles(manager.filesystem, manager.irodsTempDirPath, true, true)
	if err != nil {
		logger.WithError(err).Warnf("failed to clear staging directory %q", manager.irodsTempDirPath)
	} else {
		logger.WithError(err).Debugf("cleared staging directory %q", manager.irodsTempDirPath)
	}
}

func (manager *BundleTransferManager) startProgress() {
	if manager.showProgress {
		manager.progressWriter = GetProgressWriter(false)
		messageWidth := getProgressMessageWidth(false)

		go manager.progressWriter.Render()

		// add progress tracker callback
		manager.progressTrackerCallback = func(name string, processed int64, total int64, progressUnit progress.Units, errored bool) {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			var tracker *progress.Tracker
			if t, ok := manager.progressTrackers[name]; !ok {
				// created a new tracker if not exists
				msg := name
				if !manager.showFullPath {
					msg = GetShortPathMessage(name, messageWidth)
				}

				tracker = &progress.Tracker{
					Message: msg,
					Total:   total,
					Units:   progressUnit,
				}

				manager.progressWriter.AppendTracker(tracker)
				manager.progressTrackers[name] = tracker
			} else {
				tracker = t
			}

			if processed >= 0 {
				tracker.SetValue(processed)
			}

			if errored {
				tracker.MarkAsErrored()
			} else if processed >= total {
				tracker.MarkAsDone()
			}
		}
	}
}

func (manager *BundleTransferManager) endProgress() {
	if manager.progressWriter != nil {
		manager.mutex.Lock()

		for _, tracker := range manager.progressTrackers {
			if manager.lastError != nil {
				tracker.MarkAsDone()
			} else {
				if !tracker.IsDone() {
					tracker.MarkAsErrored()
				}
			}
		}

		manager.mutex.Unlock()

		manager.progressWriter.Stop()
	}
}

func (manager *BundleTransferManager) Start() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "Start",
	})

	// prepare
	if !manager.filesystem.ExistsDir(manager.irodsDestPath) {
		err := manager.filesystem.MakeDir(manager.irodsDestPath, true)
		if err != nil {
			return xerrors.Errorf("failed to create destination directory %q: %w", manager.irodsDestPath, err)
		}
	}

	if !manager.filesystem.ExistsDir(manager.irodsTempDirPath) {
		err := manager.filesystem.MakeDir(manager.irodsTempDirPath, true)
		if err != nil {
			return xerrors.Errorf("failed to create temporary directory %q: %w", manager.irodsTempDirPath, err)
		}
	}

	processBundleTarChan := make(chan *Bundle, 1)
	processBundleRemoveFilesAndMakeDirsChan := make(chan *Bundle, 5)
	processBundleUploadChan := make(chan *Bundle, 5)
	processBundleExtractChan1 := make(chan *Bundle, 5)
	processBundleExtractChan2 := make(chan *Bundle, 5)

	manager.startProgress()

	// bundle --> tar --> upload                   --> extract
	//        --> remove old files & make dirs ------>

	manager.processWait.Add(1) // waits for extract thread to complete

	go func() {
		logger.Debug("start input thread")
		defer logger.Debug("exit input thread")

		defer close(processBundleTarChan)
		defer close(processBundleRemoveFilesAndMakeDirsChan)

		for bundle := range manager.pendingBundles {
			// send to tar and remove
			processBundleTarChan <- bundle
			processBundleRemoveFilesAndMakeDirsChan <- bundle
		}
	}()

	// process bundle - tar
	go func() {
		logger.Debug("start bundle thread")
		defer logger.Debug("exit bundle thread")

		defer close(processBundleUploadChan)

		for bundle := range processBundleTarChan {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont {
				err := manager.processBundleTar(bundle)
				if err != nil {
					// mark error
					manager.mutex.Lock()
					manager.lastError = err
					manager.mutex.Unlock()

					bundle.LastError = err
					bundle.LastErrorTaskName = BundleTaskNameTar

					logger.Error(err)
					// don't stop here
				} else {
					bundle.CreatedTarball = true
				}
			}

			processBundleUploadChan <- bundle
		}
	}()

	// process bundle - upload
	go func() {
		logger.Debug("start transfer thread")
		defer logger.Debug("exit transfer thread")

		currentUploadThreads := 0

		defer close(processBundleExtractChan1)

		threadWaiter := sync.WaitGroup{}

		for bundle := range processBundleUploadChan {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont {
				threadsRequired := bundle.GetThreadsRequired()

				manager.mutex.Lock()
				if currentUploadThreads > 0 {
					for currentUploadThreads+threadsRequired > manager.maxTotalUploadThreads {
						// exceed max threads, wait
						logger.Debugf("waiting for other transfers to complete - current %d, max %d", currentUploadThreads, manager.maxTotalUploadThreads)

						manager.availableThreadWaitCondition.Wait()
					}
				}

				threadWaiter.Add(1)

				currentUploadThreads += threadsRequired
				logger.Debugf("# threads : %d, max %d", currentUploadThreads, manager.maxTotalUploadThreads)

				go func(pbundle *Bundle) {
					defer threadWaiter.Done()

					err := manager.processBundleUpload(pbundle)
					if err != nil {
						// mark error
						manager.mutex.Lock()
						manager.lastError = err
						manager.mutex.Unlock()

						pbundle.LastError = err
						pbundle.LastErrorTaskName = BundleTaskNameUpload

						logger.Error(err)
						// don't stop here
					} else {
						pbundle.Uploaded = true
					}

					manager.mutex.Lock()
					currentUploadThreads -= threadsRequired
					manager.availableThreadWaitCondition.Broadcast()
					manager.mutex.Unlock()

					logger.Debugf("# threads : %d, max %d", currentUploadThreads, manager.maxTotalUploadThreads)

					processBundleExtractChan1 <- pbundle
				}(bundle)

				manager.mutex.Unlock()
			}
		}

		threadWaiter.Wait()
	}()

	// process bundle - remove stale files and create new dirs
	go func() {
		logger.Debug("start directory create thread")
		defer logger.Debug("exit directory create thread")

		defer close(processBundleExtractChan2)

		for bundle := range processBundleRemoveFilesAndMakeDirsChan {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont {
				err := manager.processBundleRemoveFilesAndMakeDirs(bundle)
				if err != nil {
					// mark error
					manager.mutex.Lock()
					manager.lastError = err
					manager.mutex.Unlock()

					bundle.LastError = err
					bundle.LastErrorTaskName = BundleTaskNameRemoveFilesAndMakeDirs

					logger.Error(err)
					// don't stop here
				} else {
					bundle.MadeDir = true
				}
			}

			processBundleExtractChan2 <- bundle
		}
	}()

	// process bundle - extract
	go func() {
		logger.Debug("start extract thread")
		defer logger.Debug("exit extract thread")

		processBundleExtractChan1Closed := false
		processBundleExtractChan2Closed := false

		for {
			select {
			case bundle1, ok1 := <-processBundleExtractChan1:
				if bundle1 != nil {
					cont := true

					manager.mutex.RLock()
					if manager.lastError != nil {
						cont = false
					}
					manager.mutex.RUnlock()

					if cont {
						if bundle1.MadeDir && bundle1.Uploaded {
							// ready to extract
							err := manager.processBundleExtract(bundle1)
							if err != nil {
								// mark error
								manager.mutex.Lock()
								manager.lastError = err
								manager.mutex.Unlock()

								bundle1.LastError = err
								bundle1.LastErrorTaskName = BundleTaskNameExtract

								logger.Error(err)
								// don't stop here
							} else {
								atomic.AddInt64(&manager.bundlesDoneCounter, 1)
								bundle1.ExtractedTarball = true
							}
						}
					}
				}

				if !ok1 {
					processBundleExtractChan1Closed = true
				}
			case bundle2, ok2 := <-processBundleExtractChan2:
				if bundle2 != nil {
					cont := true

					manager.mutex.RLock()
					if manager.lastError != nil {
						cont = false
					}
					manager.mutex.RUnlock()

					if cont {
						if bundle2.MadeDir && bundle2.Uploaded {
							// ready to extract
							err := manager.processBundleExtract(bundle2)
							if err != nil {
								// mark error
								manager.mutex.Lock()
								manager.lastError = err
								manager.mutex.Unlock()

								bundle2.LastError = err
								bundle2.LastErrorTaskName = BundleTaskNameExtract

								logger.Error(err)
								// don't stop here
							} else {
								atomic.AddInt64(&manager.bundlesDoneCounter, 1)
								bundle2.ExtractedTarball = true
							}
						}
					}
				}

				if !ok2 {
					processBundleExtractChan2Closed = true
				}
			}

			if processBundleExtractChan1Closed && processBundleExtractChan2Closed {
				manager.processWait.Done()
				return
			}
		}
	}()

	go func() {
		defer manager.endProgress()
		manager.processWait.Wait()
	}()

	return nil
}

func (manager *BundleTransferManager) processBundleRemoveFilesAndMakeDirs(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleRemoveFilesAndMakeDirs",
	})

	if len(bundle.Entries) == 0 {
		logger.Debugf("skip removing files and making dirs in the bundle %d, empty bundle", bundle.Index)
		return nil
	}

	// remove files in the bundle if they exist in iRODS
	logger.Debugf("deleting exising data objects and creating new collections in the bundle %d", bundle.Index)

	progressName := manager.getProgressName(bundle, BundleTaskNameRemoveFilesAndMakeDirs)

	totalFileNum := int64(len(bundle.Entries))
	processedFiles := int64(0)

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	for _, bundleEntry := range bundle.Entries {
		entry, err := manager.filesystem.Stat(bundleEntry.IRODSPath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
				return xerrors.Errorf("failed to stat data object or collection %q: %w", bundleEntry.IRODSPath, err)
			}
		}

		if entry != nil {
			if entry.IsDir() {
				if !bundleEntry.Dir {
					logger.Debugf("deleting exising collection %q", bundleEntry.IRODSPath)
					err := manager.filesystem.RemoveDir(bundleEntry.IRODSPath, true, true)
					if err != nil {
						manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
						return xerrors.Errorf("failed to delete existing collection %q: %w", bundleEntry.IRODSPath, err)
					}
				}
			} else {
				// file
				logger.Debugf("deleting exising data object %q", bundleEntry.IRODSPath)

				err := manager.filesystem.RemoveFile(bundleEntry.IRODSPath, true)
				if err != nil {
					manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
					return xerrors.Errorf("failed to delete existing data object %q: %w", bundleEntry.IRODSPath, err)
				}
			}
		}

		processedFiles++
		manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, false)
	}

	logger.Debugf("deleted exising data objects in the bundle %d", bundle.Index)
	return nil
}

func (manager *BundleTransferManager) processBundleTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleTar",
	})

	if len(bundle.Entries) == 0 {
		logger.Debugf("skip creating a tarball for bundle %d, empty bundle", bundle.Index)
		return nil
	}

	logger.Debugf("creating a tarball for bundle %d to %q", bundle.Index, bundle.LocalBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameTar)

	totalFileNum := int64(len(bundle.Entries))

	callbackTar := func(processed int64, total int64) {
		manager.progress(progressName, processed, total, progress.UnitsDefault, false)
	}

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	if !bundle.RequireTar() {
		// no tar, so pass this step
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		logger.Debugf("skip creating a tarball for bundle %d to %q", bundle.Index, bundle.LocalBundlePath)
		return nil
	}

	entries := make([]string, len(bundle.Entries))
	for idx, entry := range bundle.Entries {
		entries[idx] = entry.LocalPath
	}

	err := Tar(manager.localBundleRootPath, entries, bundle.LocalBundlePath, callbackTar)
	if err != nil {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
		return xerrors.Errorf("failed to create a tarball for bundle %d to %q (bundle root %q): %w", bundle.Index, bundle.LocalBundlePath, bundle.manager.localBundleRootPath, err)
	}

	manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)

	logger.Debugf("created a tarball for bundle %d to %q", bundle.Index, bundle.LocalBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleUpload(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleUpload",
	})

	if len(bundle.Entries) == 0 {
		logger.Debugf("skip uploading bundle %d, empty bundle", bundle.Index)
		return nil
	}

	logger.Debugf("uploading bundle %d to %q", bundle.Index, bundle.IRODSBundlePath)

	if bundle.RequireTar() {
		return manager.processBundleUploadWithTar(bundle)
	}

	return manager.processBundleUploadWithoutTar(bundle)
}

func (manager *BundleTransferManager) processBundleUploadWithTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleUploadWithTar",
	})

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	callbackPut := func(processed int64, total int64) {
		manager.progress(progressName, processed, total, progress.UnitsBytes, false)
	}

	// check local bundle file
	localBundleStat, err := os.Stat(bundle.LocalBundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return irodsclient_types.NewFileNotFoundError(bundle.LocalBundlePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", bundle.LocalBundlePath, err)
	}

	// check irods bundle file of previous run
	bundleEntry, err := manager.filesystem.StatFile(bundle.IRODSBundlePath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return xerrors.Errorf("failed to stat existing bundle %q: %w", bundle.IRODSBundlePath, err)
		}
	} else {
		if bundleEntry.Size == localBundleStat.Size() {
			// same file exist
			manager.progress(progressName, bundle.Size, bundle.Size, progress.UnitsBytes, false)
			// remove local bundle file
			os.Remove(bundle.LocalBundlePath)
			logger.Debugf("skip uploading bundle %d to %q, file already exists", bundle.Index, bundle.IRODSBundlePath)
			return nil
		}
	}

	logger.Debugf("uploading bundle %d to %q, size %d", bundle.Index, bundle.IRODSBundlePath, localBundleStat.Size())

	notes := []string{}

	// determine how to upload
	startTime := time.Now()
	transferMode := manager.determineTransferMode(bundle.Size)
	threadsRequired := bundle.GetThreadsRequired()
	switch transferMode {
	case TransferModeRedirect:
		_, err = manager.filesystem.UploadFileRedirectToResource(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", threadsRequired, false, manager.verifyChecksum, manager.verifyChecksum, false, callbackPut)
		notes = append(notes, "redirect-to-resource", "bundle", fmt.Sprintf("%d threads", threadsRequired))
	case TransferModeICAT:
		fallthrough
	default:
		_, err = manager.filesystem.UploadFileParallel(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", threadsRequired, false, manager.verifyChecksum, manager.verifyChecksum, false, callbackPut)
		notes = append(notes, "icat", "bundle", fmt.Sprintf("%d threads", threadsRequired))
	}

	if err != nil {
		manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
		return xerrors.Errorf("failed to upload bundle %d to %q: %w", bundle.Index, bundle.IRODSBundlePath, err)
	}

	endTime := time.Now()
	notes = append(notes, fmt.Sprintf("bundle_idx:%d", bundle.Index))
	notes = append(notes, fmt.Sprintf("bundle_path:%s", bundle.IRODSBundlePath))

	for _, bundleEntry := range bundle.Entries {
		uploadResult := irodsclient_fs.FileTransferResult{
			IRODSPath: bundleEntry.IRODSPath,
			IRODSSize: bundleEntry.Size,
			LocalPath: bundleEntry.LocalPath,
			LocalSize: bundleEntry.Size,
			StartTime: startTime,
			EndTime:   endTime,
		}

		err = manager.transferReportManager.AddTransfer(&uploadResult, TransferMethodBput, err, notes)
		if err != nil {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}
	}

	// remove local bundle file
	os.Remove(bundle.LocalBundlePath)

	logger.Debugf("uploaded bundle %d to %q", bundle.Index, bundle.IRODSBundlePath)

	return nil
}

func (manager *BundleTransferManager) processBundleUploadWithoutTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleUploadWithoutTar",
	})

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	fileProgress := make([]int64, len(bundle.Entries))

	manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, false)

	for fileIdx, file := range bundle.Entries {
		callbackPut := func(processed int64, total int64) {
			fileProgress[fileIdx] = processed

			progressSum := int64(0)
			for _, progress := range fileProgress {
				progressSum += progress
			}

			manager.progress(progressName, progressSum, bundle.Size, progress.UnitsBytes, false)
		}

		if file.Dir {
			// make dir
			err := manager.filesystem.MakeDir(file.IRODSPath, true)
			if err != nil {
				manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
				return xerrors.Errorf("failed to upload a directory %q in bundle %d to %q: %w", file.LocalPath, bundle.Index, file.IRODSPath, err)
			}

			now := time.Now()
			reportFile := &TransferReportFile{
				Method:     TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: file.LocalPath,
				DestPath:   file.IRODSPath,
				Notes:      []string{"directory"},
			}

			manager.transferReportManager.AddFile(reportFile)

			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, false)
			logger.Debugf("uploaded a directory %q in bundle %d to %q", file.LocalPath, bundle.Index, file.IRODSPath)
			continue
		}

		// file
		parentDir := path.Dir(file.IRODSPath)
		if !manager.filesystem.ExistsDir(parentDir) {
			// if parent dir does not exist, create
			err := manager.filesystem.MakeDir(parentDir, true)
			if err != nil {
				manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
				return xerrors.Errorf("failed to create a directory %q to upload file %q in bundle %d to %q: %w", parentDir, file.LocalPath, bundle.Index, file.IRODSPath, err)
			}
		}

		var uploadResult *irodsclient_fs.FileTransferResult
		notes := []string{}
		var err error

		// determine how to upload
		transferMode := manager.determineTransferMode(bundle.Size)
		threadsRequired := manager.calculateThreadForFileTransfer(file.Size)
		switch transferMode {
		case TransferModeRedirect:
			uploadResult, err = manager.filesystem.UploadFileRedirectToResource(file.LocalPath, file.IRODSPath, "", threadsRequired, false, true, true, false, callbackPut)
			notes = append(notes, "redirect-to-resource", "no-bundle", fmt.Sprintf("%d threads", threadsRequired))
		case TransferModeICAT:
			fallthrough
		default:
			uploadResult, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", threadsRequired, false, true, true, false, callbackPut)
			notes = append(notes, "icat", "no-bundle", fmt.Sprintf("%d threads", threadsRequired))
		}

		if err != nil {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return xerrors.Errorf("failed to upload file %q in bundle %d to %q: %w", file.LocalPath, bundle.Index, file.IRODSPath, err)
		}

		notes = append(notes, fmt.Sprintf("bundle_idx:%d", bundle.Index))

		err = manager.transferReportManager.AddTransfer(uploadResult, TransferMethodBput, err, notes)
		if err != nil {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}

		manager.progress(progressName, file.Size, bundle.Size, progress.UnitsBytes, false)
		logger.Debugf("uploaded file %q in bundle %d to %q", file.LocalPath, bundle.Index, file.IRODSPath)
	}

	logger.Debugf("uploaded files in bundle %d to %q", bundle.Index, bundle.IRODSBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleExtract(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleExtract",
	})

	if len(bundle.Entries) == 0 {
		logger.Debugf("skip extracting bundle %d, empty bundle", bundle.Index)
		return nil
	}

	logger.Debugf("extracting bundle %d at %q", bundle.Index, bundle.IRODSBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameExtract)

	totalFileNum := int64(len(bundle.Entries))

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	if bundle.RequireTar() {
		err := manager.filesystem.ExtractStructFile(bundle.IRODSBundlePath, manager.irodsDestPath, "", irodsclient_types.TAR_FILE_DT, true, !manager.noBulkRegistration)
		if err != nil {
			manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
			return xerrors.Errorf("failed to extract bundle %d at %q to %q: %w", bundle.Index, bundle.IRODSBundlePath, manager.irodsDestPath, err)
		}

		// remove irods bundle file
		logger.Debugf("removing bundle %d at %q", bundle.Index, bundle.IRODSBundlePath)
		manager.filesystem.RemoveFile(bundle.IRODSBundlePath, true)
	} else {
		// no tar, so pass this step
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		logger.Debugf("skip extracting bundle %d at %q", bundle.Index, bundle.IRODSBundlePath)
	}

	manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)

	now := time.Now()

	for _, file := range bundle.Entries {
		reportFile := &TransferReportFile{
			Method:     TransferMethodBput,
			StartAt:    now,
			EndAt:      now,
			SourcePath: file.LocalPath,
			SourceSize: file.Size,

			DestPath: file.IRODSPath,
			DestSize: file.Size,
			Notes:    []string{"bundle_extracted"},
		}

		manager.transferReportManager.AddFile(reportFile)
	}

	logger.Debugf("extracted bundle %d at %q to %q", bundle.Index, bundle.IRODSBundlePath, manager.irodsDestPath)
	return nil
}

func (manager *BundleTransferManager) getProgressName(bundle *Bundle, taskName string) string {
	return fmt.Sprintf("bundle %d - %q", bundle.Index, taskName)
}

func (manager *BundleTransferManager) calculateThreadForBundleTransfer(bundle *Bundle) int {
	if bundle.RequireTar() {
		return manager.calculateThreadForFileTransfer(bundle.Size)
	}

	maxThreads := 1
	for _, entry := range bundle.Entries {
		curThreads := manager.calculateThreadForFileTransfer(entry.Size)
		if curThreads > maxThreads {
			maxThreads = curThreads
		}
	}
	return maxThreads
}

func (manager *BundleTransferManager) calculateThreadForFileTransfer(size int64) int {
	threads := CalculateThreadForTransferJob(size, manager.maxUploadThreadsPerFile)

	// determine how to upload
	if manager.maxTotalUploadThreads == 1 {
		return 1
	} else if manager.icat && !manager.filesystem.SupportParallelUpload() {
		return 1
	} else if manager.redirectToResource || manager.icat {
		return threads
	}

	//if size < RedirectToResourceMinSize && !manager.filesystem.SupportParallelUpload() {
	//	// icat
	//	return 1
	//}

	if !manager.filesystem.SupportParallelUpload() {
		return 1
	}

	return threads
}

func (manager *BundleTransferManager) determineTransferMode(size int64) TransferMode {
	if manager.redirectToResource {
		return TransferModeRedirect
	} else if manager.icat {
		return TransferModeICAT
	}

	// sysconfig
	systemConfig := GetSystemConfig()
	if systemConfig != nil && systemConfig.AdditionalConfig != nil {
		if systemConfig.AdditionalConfig.TransferMode.Valid() {
			return systemConfig.AdditionalConfig.TransferMode
		}
	}

	// auto
	//if size >= RedirectToResourceMinSize {
	//	return TransferModeRedirect
	//}

	return TransferModeICAT
}

func CleanUpOldLocalBundles(localTempDirPath string, force bool) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "CleanUpOldLocalBundles",
	})

	logger.Debugf("clearing local bundle files in %q", localTempDirPath)

	entries, err := os.ReadDir(localTempDirPath)
	if err != nil {
		logger.WithError(err).Warnf("failed to read a local temp directory %q", localTempDirPath)
		return
	}

	bundleEntries := []string{}
	for _, entry := range entries {
		// filter only bundle files
		if IsBundleFilename(entry.Name()) {
			fullPath := filepath.Join(localTempDirPath, entry.Name())
			bundleEntries = append(bundleEntries, fullPath)
		}
	}

	deletedCount := 0
	for _, entry := range bundleEntries {
		if force {
			logger.Debugf("deleting old local bundle %q", entry)
			removeErr := os.Remove(entry)
			if removeErr != nil {
				logger.WithError(removeErr).Warnf("failed to remove old local bundle %q", entry)
			}
		} else {
			// ask
			del := InputYN(fmt.Sprintf("removing old local bundle file %q found. Delete?", entry))
			if del {
				logger.Debugf("deleting old local bundle %q", entry)

				removeErr := os.Remove(entry)
				if removeErr != nil {
					logger.WithError(removeErr).Warnf("failed to remove old local bundle %q", entry)
				} else {
					deletedCount++
				}
			}
		}
	}

	Printf("deleted %d old local bundles in %q\n", deletedCount, localTempDirPath)
	logger.Debugf("deleted %d old local bundles in %q", deletedCount, localTempDirPath)
}

func CleanUpOldIRODSBundles(fs *irodsclient_fs.FileSystem, stagingPath string, removeDir bool, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "CleanUpOldIRODSBundles",
	})

	logger.Debugf("cleaning up old irods bundle files in %q", stagingPath)

	if !fs.ExistsDir(stagingPath) {
		return xerrors.Errorf("staging dir %q does not exist", stagingPath)
	}

	entries, err := fs.List(stagingPath)
	if err != nil {
		return xerrors.Errorf("failed to list %q: %w", stagingPath, err)
	}

	deletedCount := 0
	for _, entry := range entries {
		// filter only bundle files
		if entry.Type == irodsclient_fs.FileEntry {
			if IsBundleFilename(entry.Name) {
				logger.Debugf("deleting old irods bundle %q", entry.Path)
				removeErr := fs.RemoveFile(entry.Path, force)
				if removeErr != nil {
					return xerrors.Errorf("failed to remove bundle file %q: %w", entry.Path, removeErr)
				} else {
					deletedCount++
				}
			}
		}
	}

	Printf("deleted %d old irods bundles in %q\n", deletedCount, stagingPath)
	logger.Debugf("deleted %d old irods bundles in %q", deletedCount, stagingPath)

	if removeDir {
		if IsStagingDirInTargetPath(stagingPath) {
			rmdirErr := fs.RemoveDir(stagingPath, true, force)
			if rmdirErr != nil {
				return xerrors.Errorf("failed to remove staging directory %q: %w", stagingPath, rmdirErr)
			}
		}
	}

	return nil
}

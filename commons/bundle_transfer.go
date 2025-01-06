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
	BundleTaskNameRemoveFilesAndMakeDirs string = "Cleaning & making dirs"
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

	Completed bool
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

		Completed: false,
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

func (bundle *Bundle) SetCompleted() {
	bundle.Completed = true
}

type BundleTransferManager struct {
	// moved to top to avoid 64bit alignment issue
	bundlesScheduledCounter int64
	bundlesDoneCounter      int64

	account                 *irodsclient_types.IRODSAccount
	filesystem              *irodsclient_fs.FileSystem
	transferReportManager   *TransferReportManager
	irodsDestPath           string
	currentBundle           *Bundle
	nextBundleIndex         int64
	pendingBundles          chan *Bundle
	bundles                 []*Bundle
	localBundleRootPath     string
	minBundleFileNum        int
	maxBundleFileNum        int
	maxBundleFileSize       int64
	singleThreaded          bool
	uploadThreadNum         int
	redirectToResource      bool
	useIcat                 bool
	localTempDirPath        string
	irodsTempDirPath        string
	noBulkRegistration      bool
	showProgress            bool
	showFullPath            bool
	progressWriter          progress.Writer
	progressTrackers        map[string]*progress.Tracker
	progressTrackerCallback ProgressTrackerCallback
	lastError               error
	mutex                   sync.RWMutex

	scheduleWait sync.WaitGroup
	transferWait sync.WaitGroup
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(account *irodsclient_types.IRODSAccount, fs *irodsclient_fs.FileSystem, transferReportManager *TransferReportManager, irodsDestPath string, localBundleRootPath string, minBundleFileNum int, maxBundleFileNum int, maxBundleFileSize int64, singleThreaded bool, uploadThreadNum int, redirectToResource bool, useIcat bool, localTempDirPath string, irodsTempDirPath string, noBulkReg bool, showProgress bool, showFullPath bool) *BundleTransferManager {
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
		singleThreaded:          singleThreaded,
		uploadThreadNum:         uploadThreadNum,
		redirectToResource:      redirectToResource,
		useIcat:                 useIcat,
		localTempDirPath:        localTempDirPath,
		irodsTempDirPath:        irodsTempDirPath,
		noBulkRegistration:      noBulkReg,
		showProgress:            showProgress,
		showFullPath:            showFullPath,
		progressWriter:          nil,
		progressTrackers:        map[string]*progress.Tracker{},
		progressTrackerCallback: nil,
		lastError:               nil,
		mutex:                   sync.RWMutex{},
		scheduleWait:            sync.WaitGroup{},
		transferWait:            sync.WaitGroup{},

		bundlesScheduledCounter: 0,
		bundlesDoneCounter:      0,
	}

	if manager.maxBundleFileNum <= 0 {
		manager.maxBundleFileNum = MaxBundleFileNumDefault
	}

	if manager.minBundleFileNum <= 0 {
		manager.minBundleFileNum = MinBundleFileNumDefault
	}

	if manager.uploadThreadNum > UploadThreadNumMax {
		manager.uploadThreadNum = UploadThreadNumMax
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
			manager.transferWait.Add(1)
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
		manager.transferWait.Add(1)
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
	manager.transferWait.Wait()

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

func (manager *BundleTransferManager) Start() {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "Start",
	})

	processBundleTarChan := make(chan *Bundle, 1)
	processBundleRemoveFilesAndMakeDirsChan := make(chan *Bundle, 5)
	processBundleUploadChan := make(chan *Bundle, 5)
	processBundleExtractChan1 := make(chan *Bundle, 5)
	processBundleExtractChan2 := make(chan *Bundle, 5)

	manager.startProgress()

	// bundle --> tar --> upload                   --> extract
	//        --> remove old files & make dirs ------>

	go func() {
		logger.Debug("start input thread")
		defer logger.Debug("exit input thread")

		defer close(processBundleTarChan)
		defer close(processBundleRemoveFilesAndMakeDirsChan)

		if !manager.filesystem.ExistsDir(manager.irodsDestPath) {
			err := manager.filesystem.MakeDir(manager.irodsDestPath, true)
			if err != nil {
				// mark error
				manager.mutex.Lock()
				manager.lastError = err
				manager.mutex.Unlock()

				logger.Error(err)
				// don't stop here
			}
		}

		if !manager.filesystem.ExistsDir(manager.irodsTempDirPath) {
			err := manager.filesystem.MakeDir(manager.irodsTempDirPath, true)
			if err != nil {
				// mark error
				manager.mutex.Lock()
				manager.lastError = err
				manager.mutex.Unlock()

				logger.Error(err)
				// don't stop here
			}
		}

		for bundle := range manager.pendingBundles {
			// send to tar and remove
			processBundleTarChan <- bundle
			processBundleRemoveFilesAndMakeDirsChan <- bundle
			// don't stop here
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

			if cont && len(bundle.Entries) > 0 {
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
				}
			}

			processBundleUploadChan <- bundle
		}
	}()

	// process bundle - upload
	funcAsyncUpload := func(id int, wg *sync.WaitGroup) {
		logger.Debugf("start transfer thread %d", id)
		defer logger.Debugf("exit transfer thread %d", id)

		defer wg.Done()

		for {
			bundle, ok := <-processBundleUploadChan
			if ok {
				cont := true

				manager.mutex.RLock()
				if manager.lastError != nil {
					cont = false
				}
				manager.mutex.RUnlock()

				if cont && len(bundle.Entries) > 0 {
					err := manager.processBundleUpload(bundle)
					if err != nil {
						// mark error
						manager.mutex.Lock()
						manager.lastError = err
						manager.mutex.Unlock()

						bundle.LastError = err
						bundle.LastErrorTaskName = BundleTaskNameUpload

						logger.Error(err)
						// don't stop here
					}
				}

				processBundleExtractChan1 <- bundle
			} else {
				return
			}
		}
	}

	waitAsyncUpload := sync.WaitGroup{}
	for i := 0; i < manager.uploadThreadNum; i++ {
		waitAsyncUpload.Add(1)
		go funcAsyncUpload(i, &waitAsyncUpload)
	}

	go func() {
		waitAsyncUpload.Wait()
		close(processBundleExtractChan1)
	}()

	// process bundle - remove stale files and create new dirs
	go func() {
		logger.Debug("start stale file remove and directory create thread")
		defer logger.Debug("exit stale file remove and directory create thread")

		defer close(processBundleExtractChan2)

		for bundle := range processBundleRemoveFilesAndMakeDirsChan {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont && len(bundle.Entries) > 0 {
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
				}
			}

			processBundleExtractChan2 <- bundle
		}
	}()

	// process bundle - extract
	// order may be different
	removeTaskCompleted := map[int64]int{}
	removeTaskCompletedMutex := sync.Mutex{}

	funcAsyncExtract := func(id int, wg *sync.WaitGroup) {
		logger.Debugf("start extract thread %d", id)
		defer logger.Debugf("exit extract thread %d", id)

		defer wg.Done()

		for {
			select {
			case bundle1, ok1 := <-processBundleExtractChan1:
				if bundle1 != nil {
					removeTaskCompletedMutex.Lock()
					if _, ok := removeTaskCompleted[bundle1.Index]; ok {
						// has it
						delete(removeTaskCompleted, bundle1.Index)
						removeTaskCompletedMutex.Unlock()

						cont := true

						manager.mutex.RLock()
						if manager.lastError != nil {
							cont = false
						}
						manager.mutex.RUnlock()

						if cont && len(bundle1.Entries) > 0 {
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
							}

						} else {
							if bundle1.RequireTar() {
								// remove irods bundle file
								manager.filesystem.RemoveFile(bundle1.IRODSBundlePath, true)
							}
						}

						manager.transferWait.Done()
					} else {
						removeTaskCompleted[bundle1.Index] = 1
						removeTaskCompletedMutex.Unlock()
					}
				}

				if !ok1 {
					processBundleExtractChan1 = nil
				}

			case bundle2, ok2 := <-processBundleExtractChan2:
				if bundle2 != nil {
					removeTaskCompletedMutex.Lock()
					if _, ok := removeTaskCompleted[bundle2.Index]; ok {
						// has it
						delete(removeTaskCompleted, bundle2.Index)
						removeTaskCompletedMutex.Unlock()

						cont := true

						manager.mutex.RLock()
						if manager.lastError != nil {
							cont = false
						}
						manager.mutex.RUnlock()

						if cont && len(bundle2.Entries) > 0 {
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
							}
						} else {
							if bundle2.RequireTar() {
								// remove irods bundle file
								manager.filesystem.RemoveFile(bundle2.IRODSBundlePath, true)
							}
						}

						manager.transferWait.Done()
					} else {
						removeTaskCompleted[bundle2.Index] = 1
						removeTaskCompletedMutex.Unlock()
					}
				}

				if !ok2 {
					processBundleExtractChan2 = nil
				}
			}

			if processBundleExtractChan1 == nil && processBundleExtractChan2 == nil {
				return
			}
		}
	}

	waitAsyncExtract := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		waitAsyncExtract.Add(1)
		go funcAsyncExtract(i, &waitAsyncExtract)
	}

	go func() {
		waitAsyncExtract.Wait()

		manager.endProgress()
	}()
}

func (manager *BundleTransferManager) processBundleRemoveFilesAndMakeDirs(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleRemoveFilesAndMakeDirs",
	})

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

	logger.Debugf("uploading bundle %d to %q", bundle.Index, bundle.IRODSBundlePath)

	// determine how to download
	if manager.singleThreaded || manager.uploadThreadNum == 1 {
		_, err = manager.filesystem.UploadFile(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", false, true, true, callbackPut)
	} else if manager.redirectToResource {
		_, err = manager.filesystem.UploadFileParallelRedirectToResource(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", 0, false, true, true, callbackPut)
	} else if manager.useIcat {
		_, err = manager.filesystem.UploadFileParallel(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", 0, false, true, true, callbackPut)
	} else {
		// auto
		if bundle.Size >= RedirectToResourceMinSize {
			// redirect-to-resource
			_, err = manager.filesystem.UploadFileParallelRedirectToResource(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", 0, false, true, true, callbackPut)
		} else {
			_, err = manager.filesystem.UploadFileParallel(bundle.LocalBundlePath, bundle.IRODSBundlePath, "", 0, false, false, false, callbackPut)
		}
	}

	if err != nil {
		manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
		return xerrors.Errorf("failed to upload bundle %d to %q: %w", bundle.Index, bundle.IRODSBundlePath, err)
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

		// determine how to download
		var err error
		if manager.singleThreaded || manager.uploadThreadNum == 1 {
			uploadResult, err = manager.filesystem.UploadFile(file.LocalPath, file.IRODSPath, "", false, true, true, callbackPut)
			notes = append(notes, "icat", "single-thread")
		} else if manager.redirectToResource {
			uploadResult, err = manager.filesystem.UploadFileParallelRedirectToResource(file.LocalPath, file.IRODSPath, "", 0, false, true, true, callbackPut)
			notes = append(notes, "redirect-to-resource")
		} else if manager.useIcat {
			uploadResult, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, false, true, true, callbackPut)
			notes = append(notes, "icat", "multi-thread")
		} else {
			// auto
			if bundle.Size >= RedirectToResourceMinSize {
				// redirect-to-resource
				uploadResult, err = manager.filesystem.UploadFileParallelRedirectToResource(file.LocalPath, file.IRODSPath, "", 0, false, true, true, callbackPut)
				notes = append(notes, "redirect-to-resource")
			} else {
				uploadResult, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, false, true, true, callbackPut)
				notes = append(notes, "icat", "multi-thread")
			}
		}

		if err != nil {
			manager.progress(progressName, 0, bundle.Size, progress.UnitsBytes, true)
			return xerrors.Errorf("failed to upload file %q in bundle %d to %q: %w", file.LocalPath, bundle.Index, file.IRODSPath, err)
		}

		err = manager.transferReportManager.AddTransfer(uploadResult, TransferMethodPut, err, notes)
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

	// set it done
	bundle.SetCompleted()
	atomic.AddInt64(&manager.bundlesDoneCounter, 1)

	now := time.Now()

	for _, file := range bundle.Entries {
		reportFile := &TransferReportFile{
			Method:     TransferMethodPut,
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

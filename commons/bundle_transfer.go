package commons

import (
	"bytes"
	"encoding/hex"
	"fmt"
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

	index             int64
	entries           []*BundleEntry
	size              int64
	localBundlePath   string
	irodsBundlePath   string
	lastError         error
	lastErrorTaskName string

	done bool
}

func newBundle(manager *BundleTransferManager) (*Bundle, error) {
	bundle := &Bundle{
		manager:           manager,
		index:             manager.getNextBundleIndex(),
		entries:           []*BundleEntry{},
		size:              0,
		localBundlePath:   "",
		irodsBundlePath:   "",
		lastError:         nil,
		lastErrorTaskName: "",

		done: false,
	}

	err := bundle.updateBundlePath()
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (bundle *Bundle) GetEntries() []*BundleEntry {
	return bundle.entries
}

func (bundle *Bundle) GetBundleFilename() (string, error) {
	entryStrs := []string{}

	entryStrs = append(entryStrs, "empty_bundle")

	for _, entry := range bundle.entries {
		entryStrs = append(entryStrs, entry.LocalPath)
	}

	hash, err := irodsclient_util.HashStrings(entryStrs, string(irodsclient_types.ChecksumAlgorithmMD5))
	if err != nil {
		return "", err
	}

	hexhash := hex.EncodeToString(hash)

	return GetBundleFilename(hexhash), nil
}

func (bundle *Bundle) AddFile(localPath string, size int64) error {
	irodsPath, err := bundle.manager.getTargetPath(localPath)
	if err != nil {
		return xerrors.Errorf("failed to get target path for %q: %w", localPath, err)
	}

	e := &BundleEntry{
		LocalPath: localPath,
		IRODSPath: irodsPath,
		Size:      size,
		Dir:       false,
	}

	bundle.entries = append(bundle.entries, e)
	bundle.size += size

	err = bundle.updateBundlePath()
	if err != nil {
		return err
	}

	return nil
}

func (bundle *Bundle) AddDir(localPath string) error {
	irodsPath, err := bundle.manager.getTargetPath(localPath)
	if err != nil {
		return xerrors.Errorf("failed to get target path for %q: %w", localPath, err)
	}

	e := &BundleEntry{
		LocalPath: localPath,
		IRODSPath: irodsPath,
		Size:      0,
		Dir:       true,
	}

	bundle.entries = append(bundle.entries, e)

	err = bundle.updateBundlePath()
	if err != nil {
		return err
	}

	return nil
}

func (bundle *Bundle) updateBundlePath() error {
	filename, err := bundle.GetBundleFilename()
	if err != nil {
		return xerrors.Errorf("failed to get bundle filename: %w", err)
	}

	bundle.localBundlePath = filepath.Join(bundle.manager.localTempDirPath, filename)
	bundle.irodsBundlePath = filepath.Join(bundle.manager.irodsTempDirPath, filename)
	return nil
}

func (bundle *Bundle) isFull() bool {
	return bundle.size >= bundle.manager.maxBundleFileSize || len(bundle.entries) >= bundle.manager.maxBundleFileNum
}

func (bundle *Bundle) requireTar() bool {
	return len(bundle.entries) >= MinBundleFileNumDefault
}

func (bundle *Bundle) Done() {
	bundle.done = true
}

type BundleTransferManager struct {
	filesystem              *irodsclient_fs.FileSystem
	irodsDestPath           string
	currentBundle           *Bundle
	nextBundleIndex         int64
	pendingBundles          chan *Bundle
	bundles                 []*Bundle
	inputPathMap            map[string]bool
	bundleRootPath          string
	maxBundleFileNum        int
	maxBundleFileSize       int64
	singleThreaded          bool
	uploadThreadNum         int
	redirectToResource      bool
	useIcat                 bool
	localTempDirPath        string
	irodsTempDirPath        string
	differentFilesOnly      bool
	noHashForComparison     bool
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

	bundlesScheduledCounter int64
	bundlesDoneCounter      int64
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(fs *irodsclient_fs.FileSystem, irodsDestPath string, bundleRootPath string, maxBundleFileNum int, maxBundleFileSize int64, singleThreaded bool, uploadThreadNum int, redirectToResource bool, useIcat bool, localTempDirPath string, irodsTempDirPath string, diff bool, noHash bool, noBulkReg bool, showProgress bool, showFullPath bool) *BundleTransferManager {
	cwd := GetCWD()
	home := GetHomeDir()
	zone := GetZone()
	irodsDestPath = MakeIRODSPath(cwd, home, zone, irodsDestPath)

	manager := &BundleTransferManager{
		filesystem:              fs,
		irodsDestPath:           irodsDestPath,
		currentBundle:           nil,
		nextBundleIndex:         0,
		pendingBundles:          make(chan *Bundle, 100),
		bundles:                 []*Bundle{},
		inputPathMap:            map[string]bool{},
		bundleRootPath:          "/",
		maxBundleFileNum:        maxBundleFileNum,
		maxBundleFileSize:       maxBundleFileSize,
		singleThreaded:          singleThreaded,
		uploadThreadNum:         uploadThreadNum,
		redirectToResource:      redirectToResource,
		useIcat:                 useIcat,
		localTempDirPath:        localTempDirPath,
		irodsTempDirPath:        irodsTempDirPath,
		differentFilesOnly:      diff,
		noHashForComparison:     noHash,
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

	if manager.uploadThreadNum > UploadTreadNumMax {
		manager.uploadThreadNum = UploadTreadNumMax
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

func (manager *BundleTransferManager) getTargetPath(localPath string) (string, error) {
	relPath, err := filepath.Rel(manager.bundleRootPath, localPath)
	if err != nil {
		return "", xerrors.Errorf("failed to compute relative path %q to %q: %w", localPath, manager.bundleRootPath, err)
	}

	return path.Join(manager.irodsDestPath, filepath.ToSlash(relPath)), nil
}

func (manager *BundleTransferManager) Schedule(source string, dir bool, size int64, lastModTime time.Time) error {
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
			return xerrors.Errorf("failed to create a new bundle for %q: %w", source, err)
		}

		manager.currentBundle = bundle
		logger.Debugf("assigned a new bundle %d", manager.currentBundle.index)
	}

	defer manager.mutex.Unlock()
	targePath, err := manager.getTargetPath(source)
	if err != nil {
		return xerrors.Errorf("failed to get target path for %q: %w", source, err)
	}

	MarkPathMap(manager.inputPathMap, targePath)

	if manager.differentFilesOnly {
		logger.Debugf("checking if target %q for source %q exists", targePath, source)

		if dir {
			// handle dir
			exist := manager.filesystem.ExistsDir(targePath)
			if exist {
				Printf("skip adding a directory %q to the bundle. The dir already exists!\n", source)
				logger.Debugf("skip adding a directory %q to the bundle. The dir already exists!", source)
				return nil
			}

			logger.Debugf("adding a directory %q to the bundle as it doesn't exist", source)
		} else {
			exist := manager.filesystem.ExistsFile(targePath)
			if exist {
				targetEntry, err := manager.filesystem.Stat(targePath)
				if err != nil {
					return xerrors.Errorf("failed to stat %q: %w", targePath, err)
				}

				if manager.noHashForComparison {
					if targetEntry.Size == size {
						Printf("skip adding a file %q to the bundle. The file already exists!\n", source)
						logger.Debugf("skip adding a file %q to the bundle. The file already exists!", source)
						return nil
					}

					logger.Debugf("adding a file %q to the bundle as it has different size %d != %d", source, targetEntry.Size, size)
				} else {
					if targetEntry.Size == size {
						if len(targetEntry.CheckSum) > 0 {
							// compare hash
							hash, err := irodsclient_util.HashLocalFile(source, string(targetEntry.CheckSumAlgorithm))
							if err != nil {
								return xerrors.Errorf("failed to get hash %q: %w", source, err)
							}

							if bytes.Equal(hash, targetEntry.CheckSum) {
								Printf("skip adding a file %q to the bundle. The file with the same hash already exists!\n", source)
								logger.Debugf("skip adding a file %q to the bundle. The file with the same hash already exists!", source)
								return nil
							}

							logger.Debugf("adding a file %q to the bundle as it has different hash, %q vs %q (alg %q)", source, hash, targetEntry.CheckSum, targetEntry.CheckSumAlgorithm)
						} else {
							logger.Debugf("adding a file %q to the bundle as the file in iRODS doesn't have hash yet", source)
						}
					} else {
						logger.Debugf("adding a file %q to the bundle as it has different size %d != %d", source, targetEntry.Size, size)
					}
				}
			} else {
				logger.Debugf("adding a file %q to the bundle as it doesn't exist", source)
			}
		}
	}

	if dir {
		manager.currentBundle.AddDir(source)
		logger.Debugf("> scheduled a local file bundle-upload %q", source)
		return nil
	}

	manager.currentBundle.AddFile(source, size)
	logger.Debugf("> scheduled a local file bundle-upload %q", source)
	return nil
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

func (manager *BundleTransferManager) GetInputPathMap() map[string]bool {
	return manager.inputPathMap
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
		return xerrors.Errorf("bundles '%d/%d' were canceled!", manager.bundlesDoneCounter, manager.bundlesScheduledCounter)
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

	// if the staging dir is not in target path
	entries, err := manager.filesystem.List(manager.irodsTempDirPath)
	if err != nil {
		logger.WithError(err).Warnf("failed to listing a staging directory %q", manager.irodsTempDirPath)
		return
	}

	if len(entries) == 0 {
		// empty
		// remove the dir
		err := manager.filesystem.RemoveDir(manager.irodsTempDirPath, true, true)
		if err != nil {
			logger.WithError(err).Warnf("failed to remove staging directory %q", manager.irodsTempDirPath)
			return
		}
		return
	}

	// has some files in it yet
	// ask
	deleted := 0
	for _, entry := range entries {
		del := InputYN(fmt.Sprintf("removing old bundle file %q found. Delete?", entry.Path))
		if del {
			logger.Debugf("deleting old bundle file %q", entry.Path)

			removeErr := os.Remove(entry.Path)
			if removeErr != nil {
				logger.WithError(removeErr).Warnf("failed to remove old bundle file %q", entry.Path)
			}

			deleted++
		}
	}

	// check again
	// if the staging dir is not in target path
	entries, err = manager.filesystem.List(manager.irodsTempDirPath)
	if err != nil {
		logger.WithError(err).Warnf("failed to listing a staging directory %q", manager.irodsTempDirPath)
		return
	}

	if len(entries) == 0 {
		// empty
		// remove the dir
		err := manager.filesystem.RemoveDir(manager.irodsTempDirPath, true, true)
		if err != nil {
			logger.WithError(err).Warnf("failed to remove a staging directory %q", manager.irodsTempDirPath)
			return
		}
		return
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
	if manager.showProgress {
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

			if cont && len(bundle.entries) > 0 {
				err := manager.processBundleTar(bundle)
				if err != nil {
					// mark error
					manager.mutex.Lock()
					manager.lastError = err
					manager.mutex.Unlock()

					bundle.lastError = err
					bundle.lastErrorTaskName = BundleTaskNameTar

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

				if cont && len(bundle.entries) > 0 {
					err := manager.processBundleUpload(bundle)
					if err != nil {
						// mark error
						manager.mutex.Lock()
						manager.lastError = err
						manager.mutex.Unlock()

						bundle.lastError = err
						bundle.lastErrorTaskName = BundleTaskNameUpload

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

			if cont && len(bundle.entries) > 0 {
				err := manager.processBundleRemoveFilesAndMakeDirs(bundle)
				if err != nil {
					// mark error
					manager.mutex.Lock()
					manager.lastError = err
					manager.mutex.Unlock()

					bundle.lastError = err
					bundle.lastErrorTaskName = BundleTaskNameRemoveFilesAndMakeDirs

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
					if _, ok := removeTaskCompleted[bundle1.index]; ok {
						// has it
						delete(removeTaskCompleted, bundle1.index)
						removeTaskCompletedMutex.Unlock()

						cont := true

						manager.mutex.RLock()
						if manager.lastError != nil {
							cont = false
						}
						manager.mutex.RUnlock()

						if cont && len(bundle1.entries) > 0 {
							err := manager.processBundleExtract(bundle1)
							if err != nil {
								// mark error
								manager.mutex.Lock()
								manager.lastError = err
								manager.mutex.Unlock()

								bundle1.lastError = err
								bundle1.lastErrorTaskName = BundleTaskNameExtract

								logger.Error(err)
								// don't stop here
							}

						} else {
							if bundle1.requireTar() {
								// remove irods bundle file
								manager.filesystem.RemoveFile(bundle1.irodsBundlePath, true)
							}
						}

						manager.transferWait.Done()
					} else {
						removeTaskCompleted[bundle1.index] = 1
						removeTaskCompletedMutex.Unlock()
					}
				}

				if !ok1 {
					processBundleExtractChan1 = nil
				}

			case bundle2, ok2 := <-processBundleExtractChan2:
				if bundle2 != nil {
					removeTaskCompletedMutex.Lock()
					if _, ok := removeTaskCompleted[bundle2.index]; ok {
						// has it
						delete(removeTaskCompleted, bundle2.index)
						removeTaskCompletedMutex.Unlock()

						cont := true

						manager.mutex.RLock()
						if manager.lastError != nil {
							cont = false
						}
						manager.mutex.RUnlock()

						if cont && len(bundle2.entries) > 0 {
							err := manager.processBundleExtract(bundle2)
							if err != nil {
								// mark error
								manager.mutex.Lock()
								manager.lastError = err
								manager.mutex.Unlock()

								bundle2.lastError = err
								bundle2.lastErrorTaskName = BundleTaskNameExtract

								logger.Error(err)
								// don't stop here
							}
						} else {
							if bundle2.requireTar() {
								// remove irods bundle file
								manager.filesystem.RemoveFile(bundle2.irodsBundlePath, true)
							}
						}

						manager.transferWait.Done()
					} else {
						removeTaskCompleted[bundle2.index] = 1
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
	logger.Debugf("deleting exising data objects and creating new collections in the bundle %d", bundle.index)

	progressName := manager.getProgressName(bundle, BundleTaskNameRemoveFilesAndMakeDirs)

	totalFileNum := int64(len(bundle.entries))
	processedFiles := int64(0)

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)
	}

	for _, bundleEntry := range bundle.entries {
		entry, err := manager.filesystem.Stat(bundleEntry.IRODSPath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				if manager.showProgress {
					manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
				}

				logger.Error(err)
				return xerrors.Errorf("failed to stat data object or collection %q", bundleEntry.IRODSPath)
			}
		}

		if entry != nil {
			if entry.IsDir() {
				if !bundleEntry.Dir {
					logger.Debugf("deleting exising collection %q", bundleEntry.IRODSPath)
					err := manager.filesystem.RemoveDir(bundleEntry.IRODSPath, true, true)
					if err != nil {
						if manager.showProgress {
							manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
						}

						logger.Error(err)
						return xerrors.Errorf("failed to delete existing collection %q", bundleEntry.IRODSPath)
					}
				}
			} else {
				// file
				logger.Debugf("deleting exising data object %q", bundleEntry.IRODSPath)

				err := manager.filesystem.RemoveFile(bundleEntry.IRODSPath, true)
				if err != nil {
					if manager.showProgress {
						manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
					}

					logger.Error(err)
					return xerrors.Errorf("failed to delete existing data object %q", bundleEntry.IRODSPath)
				}
			}
		}

		processedFiles++
		if manager.showProgress {
			manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, false)
		}
	}

	logger.Debugf("deleted exising data objects in the bundle %d", bundle.index)
	return nil
}

func (manager *BundleTransferManager) processBundleTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleTar",
	})

	logger.Debugf("creating a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameTar)

	totalFileNum := int64(len(bundle.entries))

	var callback func(processed int64, total int64)
	if manager.showProgress {
		callback = func(processed int64, total int64) {
			manager.progress(progressName, processed, total, progress.UnitsDefault, false)
		}
	}

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)
	}

	if !bundle.requireTar() {
		// no tar, so pass this step
		if manager.showProgress {
			manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		}

		logger.Debugf("skip - creating a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)
		return nil
	}

	entries := make([]string, len(bundle.entries))
	for idx, entry := range bundle.entries {
		entries[idx] = entry.LocalPath
	}

	err := Tar(manager.bundleRootPath, entries, bundle.localBundlePath, callback)
	if err != nil {
		if manager.showProgress {
			manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
		}

		return xerrors.Errorf("failed to create a tarball for bundle %d to %q: %w", bundle.index, bundle.localBundlePath, err)
	}

	logger.Debugf("created a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleUpload(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleUpload",
	})

	logger.Debugf("uploading bundle %d to %q", bundle.index, bundle.irodsBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	totalFileSize := bundle.size

	if bundle.requireTar() {
		var callback func(processed int64, total int64)
		if manager.showProgress {
			callback = func(processed int64, total int64) {
				manager.progress(progressName, processed, total, progress.UnitsBytes, false)
			}
		}

		haveExistingBundle := false

		bundleEntry, err := manager.filesystem.StatFile(bundle.irodsBundlePath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				if manager.showProgress {
					manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
				}

				return xerrors.Errorf("failed to stat existing bundle %q: %w", bundle.irodsBundlePath, err)
			}
		}

		if bundleEntry != nil {
			localBundleStat, err := os.Stat(bundle.localBundlePath)
			if err != nil {
				if os.IsNotExist(err) {
					return irodsclient_types.NewFileNotFoundError(bundle.localBundlePath)
				}

				return xerrors.Errorf("failed to stat %q: %w", bundle.localBundlePath, err)
			}

			if bundleEntry.Size == localBundleStat.Size() {
				// same file exist
				haveExistingBundle = true
			}
		}

		if !haveExistingBundle {
			logger.Debugf("uploading bundle %d to %q", bundle.index, bundle.irodsBundlePath)

			// determine how to download
			if manager.singleThreaded || manager.uploadThreadNum == 1 {
				_, err = manager.filesystem.UploadFile(bundle.localBundlePath, bundle.irodsBundlePath, "", false, true, true, callback)
			} else if manager.redirectToResource {
				_, err = manager.filesystem.UploadFileParallelRedirectToResource(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, true, true, callback)
			} else if manager.useIcat {
				_, err = manager.filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, true, true, callback)
			} else {
				// auto
				if bundle.size >= RedirectToResourceMinSize {
					// redirect-to-resource
					_, err = manager.filesystem.UploadFileParallelRedirectToResource(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, true, true, callback)
				} else {
					if manager.filesystem.SupportParallelUpload() {
						_, err = manager.filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, false, false, callback)
					} else {
						if bundle.size >= ParallelUploadMinSize {
							// does not support parall upload via iCAT
							// redirect-to-resource
							_, err = manager.filesystem.UploadFileParallelRedirectToResource(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, false, false, callback)
						} else {
							_, err = manager.filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, false, false, callback)
						}
					}
				}
			}

			if err != nil {
				if manager.showProgress {
					manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
				}

				return xerrors.Errorf("failed to upload bundle %d to %q: %w", bundle.index, bundle.irodsBundlePath, err)
			}

			logger.Debugf("uploaded bundle %d to %q", bundle.index, bundle.irodsBundlePath)
		} else {
			logger.Debugf("skip uploading bundle %d to %q, file already exists", bundle.index, bundle.irodsBundlePath)
		}

		// remove local bundle file
		os.Remove(bundle.localBundlePath)
		return nil
	}

	fileUploadProgress := make([]int64, len(bundle.entries))
	fileUploadProgressMutex := sync.Mutex{}

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileSize, progress.UnitsBytes, false)
	}

	for fileIdx, file := range bundle.entries {
		var callbackFileUpload func(processed int64, total int64)
		if manager.showProgress {
			callbackFileUpload = func(processed int64, total int64) {
				fileUploadProgressMutex.Lock()
				defer fileUploadProgressMutex.Unlock()

				fileUploadProgress[fileIdx] = processed

				progressSum := int64(0)
				for _, progress := range fileUploadProgress {
					progressSum += progress
				}

				manager.progress(progressName, progressSum, totalFileSize, progress.UnitsBytes, false)
			}
		}

		if !manager.filesystem.ExistsDir(path.Dir(file.IRODSPath)) {
			// if parent dir does not exist, create
			err := manager.filesystem.MakeDir(path.Dir(file.IRODSPath), true)
			if err != nil {
				if manager.showProgress {
					manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
				}

				return xerrors.Errorf("failed to create a directory %q to upload file %q in bundle %d to %q: %w", path.Dir(file.IRODSPath), file.LocalPath, bundle.index, file.IRODSPath, err)
			}
		}

		var err error
		if file.Dir {
			err = manager.filesystem.MakeDir(file.IRODSPath, true)
			if err != nil {
				if manager.showProgress {
					manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
				}

				return xerrors.Errorf("failed to upload a directory %q in bundle %d to %q: %w", file.LocalPath, bundle.index, file.IRODSPath, err)
			}

			logger.Debugf("uploaded a directory %q in bundle %d to %q", file.LocalPath, bundle.index, file.IRODSPath)
		} else {
			// determine how to download
			if manager.singleThreaded || manager.uploadThreadNum == 1 {
				_, err = manager.filesystem.UploadFile(file.LocalPath, file.IRODSPath, "", false, false, false, callbackFileUpload)
			} else if manager.redirectToResource {
				_, err = manager.filesystem.UploadFileParallelRedirectToResource(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
			} else if manager.useIcat {
				_, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
			} else {
				// auto
				if bundle.size >= RedirectToResourceMinSize {
					// redirect-to-resource
					_, err = manager.filesystem.UploadFileParallelRedirectToResource(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
				} else {
					if manager.filesystem.SupportParallelUpload() {
						_, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
					} else {
						if bundle.size >= ParallelUploadMinSize {
							// does not support parall upload via iCAT
							// redirect-to-resource
							_, err = manager.filesystem.UploadFileParallelRedirectToResource(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
						} else {
							_, err = manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, false, false, false, callbackFileUpload)
						}
					}
				}
			}

			if err != nil {
				if manager.showProgress {
					manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
				}

				return xerrors.Errorf("failed to upload file %q in bundle %d to %q: %w", file.LocalPath, bundle.index, file.IRODSPath, err)
			}

			logger.Debugf("uploaded file %q in bundle %d to %q", file.LocalPath, bundle.index, file.IRODSPath)
		}
	}

	logger.Debugf("uploaded files in bundle %d to %q", bundle.index, bundle.irodsBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleExtract(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleExtract",
	})

	logger.Debugf("extracting bundle %d at %q", bundle.index, bundle.irodsBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameExtract)

	totalFileNum := int64(len(bundle.entries))

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)
	}

	if !bundle.requireTar() {
		// no tar, so pass this step
		if manager.showProgress {
			manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		}

		logger.Debugf("skip - extracting bundle %d at %q", bundle.index, bundle.irodsBundlePath)
		return nil
	}

	err := manager.filesystem.ExtractStructFile(bundle.irodsBundlePath, manager.irodsDestPath, "", irodsclient_types.TAR_FILE_DT, true, !manager.noBulkRegistration)
	if err != nil {
		if manager.showProgress {
			manager.progress(progressName, -1, totalFileNum, progress.UnitsDefault, true)
		}

		manager.filesystem.RemoveFile(bundle.irodsBundlePath, true)
		return xerrors.Errorf("failed to extract bundle %d at %q to %q: %w", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath, err)
	}

	// remove irods bundle file
	logger.Debugf("removing bundle %d at %q", bundle.index, bundle.irodsBundlePath)
	manager.filesystem.RemoveFile(bundle.irodsBundlePath, true)

	if manager.showProgress {
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
	}

	// set it done
	bundle.Done()
	atomic.AddInt64(&manager.bundlesDoneCounter, 1)

	logger.Debugf("extracted bundle %d at %q to %q", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath)
	return nil
}

func (manager *BundleTransferManager) getProgressName(bundle *Bundle, taskName string) string {
	return fmt.Sprintf("bundle %d - %q", bundle.index, taskName)
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

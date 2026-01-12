package bundle

/*
import (
	"container/list"
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
	"github.com/cockroachdb/errors"
)

const (
	BundleTaskNameUpload  string = "Uploading"
	BundleTaskNameExtract string = "Extracting"
)

type BundleEntry struct {
	localPath string
	irodsPath string
	size      int64
	dir       bool
}

type Bundle struct {
	manager *BundleTransferManager

	index                   int64
	entries                 []BundleEntry
	size                    int64
	transferThreadsRequired int
	bundleFilename          string
	localBundlePath         string
	irodsBundlePath         string
	lastError               error
	lastErrorTaskName       string

	sealed   bool
	canceled bool
	mutex    sync.Mutex

	createdTarball   bool
	uploaded         bool
	madeDir          bool
	extractedTarball bool
}

func newBundle(manager *BundleTransferManager) *Bundle {
	return &Bundle{
		manager:                 manager,
		index:                   manager.getNextBundleIndex(),
		entries:                 []BundleEntry{},
		size:                    0,
		transferThreadsRequired: 0,
		bundleFilename:          "",
		localBundlePath:         "",
		irodsBundlePath:         "",
		lastError:               nil,
		lastErrorTaskName:       "",

		sealed:   false,
		canceled: false,

		createdTarball:   false,
		uploaded:         false,
		madeDir:          false,
		extractedTarball: false,
	}
}

func (bundle *Bundle) GetManager() *BundleTransferManager {
	return bundle.manager
}

func (bundle *Bundle) GetEntries() []BundleEntry {
	return bundle.entries
}

func (bundle *Bundle) makeBundleFilename() (string, error) {
	entryStrs := []string{}

	entryStrs = append(entryStrs, "empty_bundle")

	for _, entry := range bundle.entries {
		entryStrs = append(entryStrs, entry.localPath)
	}

	hash, err := irodsclient_util.HashStrings(entryStrs, string(irodsclient_types.ChecksumAlgorithmMD5))
	if err != nil {
		return "", err
	}

	hexhash := hex.EncodeToString(hash)

	return MakeBundleFilename(hexhash), nil
}

func (bundle *Bundle) GetBundleFilename() string {
	return bundle.bundleFilename
}

func (bundle *Bundle) GetTransferThreadsRequired() int {
	return bundle.transferThreadsRequired
}

func (bundle *Bundle) SetCanceled() {
	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	bundle.canceled = true
}

func (bundle *Bundle) IsCanceled() bool {
	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	return bundle.canceled
}

func (bundle *Bundle) Add(sourceStat fs.FileInfo, sourcePath string) error {
	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	irodsPath, err := bundle.manager.GetTargetPath(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to get target path for %q", sourcePath)
	}

	entry := BundleEntry{
		localPath: sourcePath,
		irodsPath: irodsPath,
		size:      sourceStat.Size(),
		dir:       sourceStat.IsDir(),
	}

	bundle.entries = append(bundle.entries, entry)
	if !sourceStat.IsDir() {
		bundle.size += sourceStat.Size()
	}

	return nil
}

func (bundle *Bundle) Seal() error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "Seal",
	})

	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	filename, err := bundle.makeBundleFilename()
	if err != nil {
		return errors.Wrap(err, "failed to get bundle filename")
	}

	bundle.bundleFilename = filename

	logger.Debugf("bundle local temp path %q, irods temp path %q", bundle.manager.localTempDirPath, bundle.manager.irodsTempDirPath)

	bundle.localBundlePath = filepath.Join(bundle.manager.localTempDirPath, filename)
	bundle.irodsBundlePath = path.Join(bundle.manager.irodsTempDirPath, filename)

	logger.Debugf("bundle local path %q, irods path %q", bundle.localBundlePath, bundle.irodsBundlePath)

	return nil
}

func (bundle *Bundle) IsSealed() bool {
	return bundle.sealed
}

func (bundle *Bundle) IsFull() bool {
	return bundle.size >= bundle.manager.maxBundleFileSize || len(bundle.entries) >= bundle.manager.maxBundleFileNum
}

func (bundle *Bundle) RequireTar() bool {
	return len(bundle.entries) >= bundle.manager.minBundleFileNum
}

type BundleTransferManager struct {
	// moved to top to avoid 64bit alignment issue
	bundlesScheduledCounter int64
	bundlesDoneCounter      int64
	bundlesErroredCounter   int64
	bundlesCanceledCounter  int64

	account               *irodsclient_types.IRODSAccount
	filesystem            *irodsclient_fs.FileSystem
	transferReportManager *TransferReportManager
	irodsDestPath         string

	nextBundleIndex        int64
	currentBundleToAdd     *Bundle
	pendingBundles         *list.List        // list of *Bundle
	runningBundles         map[int64]*Bundle // map of bundle index to *Bundle
	totalBundles           int
	transferWeightCapacity int
	currentWeight          int

	bundles []*Bundle

	minBundleFileNum        int
	maxBundleFileNum        int
	maxBundleFileSize       int64
	maxTotalUploadThreads   int
	maxUploadThreadsPerFile int

	localBundleRootPath string
	localTempDirPath    string
	irodsTempDirPath    string

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
func NewBundleTransferManager(account *irodsclient_types.IRODSAccount, fs *irodsclient_fs.FileSystem, transferReportManager *TransferReportManager, irodsDestPath string, localBundleRootPath string, minBundleFileNum int, maxBundleFileNum int, maxBundleFileSize int64, maxTotalUploadThreads int, maxUploadThreadsPerFile int, useIcat bool, localTempDirPath string, irodsTempDirPath string, noBulkReg bool, verifyChecksum bool, showProgress bool, showFullPath bool) *BundleTransferManager {
	cwd := GetCWD()
	home := GetHomeDir()
	zone := account.ClientZone
	irodsDestPath = MakeIRODSPath(cwd, home, zone, irodsDestPath)

	manager := &BundleTransferManager{
		account:                 account,
		filesystem:              fs,
		transferReportManager:   transferReportManager,
		irodsDestPath:           irodsDestPath,
		currentBundleToAdd:      nil,
		nextBundleIndex:         0,
		pendingBundles:          make(chan *Bundle, 100),
		bundles:                 []*Bundle{},
		localBundleRootPath:     localBundleRootPath,
		minBundleFileNum:        minBundleFileNum,
		maxBundleFileNum:        maxBundleFileNum,
		maxBundleFileSize:       maxBundleFileSize,
		maxTotalUploadThreads:   maxTotalUploadThreads,
		maxUploadThreadsPerFile: maxUploadThreadsPerFile,
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
		return "", errors.Wrapf(err, "failed to compute relative path %q to %q", localPath, manager.localBundleRootPath)
	}

	return path.Join(manager.irodsDestPath, filepath.ToSlash(relPath)), nil
}

func (manager *BundleTransferManager) Schedule(sourceStat fs.FileInfo, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "Schedule",
	})

	manager.mutex.Lock()

	// do not accept new schedule if there's an error
	if manager.lastError != nil {
		defer manager.mutex.Unlock()
		return manager.lastError
	}

	if manager.currentBundleToAdd != nil {
		// if current bundle is full, prepare a new bundle
		if manager.currentBundleToAdd.IsFull() {
			// temporarily release lock since adding to chan may block
			manager.mutex.Unlock()

			manager.pendingBundles <- manager.currentBundleToAdd
			manager.bundles = append(manager.bundles, manager.currentBundleToAdd)

			manager.mutex.Lock()
			manager.currentBundleToAdd = nil
			atomic.AddInt64(&manager.bundlesScheduledCounter, 1)
		}
	}

	if manager.currentBundleToAdd == nil {
		// add new
		bundle, err := newBundle(manager)
		if err != nil {
			return errors.Wrapf(err, "failed to create a new bundle for %q", sourcePath)
		}

		manager.currentBundleToAdd = bundle
		logger.Debugf("assigned a new bundle %d", manager.currentBundleToAdd.index)
	}

	defer manager.mutex.Unlock()

	logger.Debugf("scheduling a local file/directory bundle-upload %q", sourcePath)
	return manager.currentBundleToAdd.Add(sourceStat, sourcePath)
}

func (manager *BundleTransferManager) DoneScheduling() {
	manager.mutex.Lock()
	if manager.currentBundleToAdd != nil {
		manager.pendingBundles <- manager.currentBundleToAdd
		manager.bundles = append(manager.bundles, manager.currentBundleToAdd)
		manager.currentBundleToAdd = nil
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
		"package":  "bundle_transfer",
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
		return errors.Errorf("%d bundles were done out of %d! Some bundles failed!", manager.bundlesDoneCounter, manager.bundlesScheduledCounter)
	}

	return nil
}

func (manager *BundleTransferManager) CleanUpBundles() {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
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
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "Start",
	})

	// prepare
	if !manager.filesystem.ExistsDir(manager.irodsDestPath) {
		err := manager.filesystem.MakeDir(manager.irodsDestPath, true)
		if err != nil {
			return errors.Wrapf(err, "failed to make a destination directory %q", manager.irodsDestPath)
		}
	}

	if !manager.filesystem.ExistsDir(manager.irodsTempDirPath) {
		err := manager.filesystem.MakeDir(manager.irodsTempDirPath, true)
		if err != nil {
			return errors.Wrapf(err, "failed to make a temporary directory %q", manager.irodsTempDirPath)
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

					bundle.lastError = err
					bundle.lastErrorTaskName = BundleTaskNameTar

					logger.Error(err)
					// don't stop here
				} else {
					bundle.createdTarball = true
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
				_, threadsRequired := manager.determineTransferMethodForBundle(bundle)

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

						pbundle.lastError = err
						pbundle.lastErrorTaskName = BundleTaskNameUpload

						logger.Error(err)
						// don't stop here
					} else {
						pbundle.uploaded = true
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

					bundle.lastError = err
					bundle.lastErrorTaskName = BundleTaskNameRemoveFilesAndMakeDirs

					logger.Error(err)
					// don't stop here
				} else {
					bundle.madeDir = true
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
						if bundle1.madeDir && bundle1.uploaded {
							// ready to extract
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
							} else {
								atomic.AddInt64(&manager.bundlesDoneCounter, 1)
								bundle1.extractedTarball = true
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
						if bundle2.madeDir && bundle2.uploaded {
							// ready to extract
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
							} else {
								atomic.AddInt64(&manager.bundlesDoneCounter, 1)
								bundle2.extractedTarball = true
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
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleRemoveFilesAndMakeDirs",
	})

	if len(bundle.entries) == 0 {
		logger.Debugf("skip removing files and making dirs in the bundle %d, empty bundle", bundle.index)
		return nil
	}

	// remove files in the bundle if they exist in iRODS
	logger.Debugf("deleting exising data objects and creating new collections in the bundle %d", bundle.index)

	progressName := manager.getProgressName(bundle, BundleTaskNameRemoveFilesAndMakeDirs)

	totalFileNum := int64(len(bundle.entries))
	processedFiles := int64(0)

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	for _, bundleEntry := range bundle.entries {
		entry, err := manager.filesystem.Stat(bundleEntry.irodsPath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
				return errors.Wrapf(err, "failed to stat data object or collection %q", bundleEntry.irodsPath)
			}
		}

		if entry != nil {
			if entry.IsDir() {
				if !bundleEntry.dir {
					logger.Debugf("deleting exising collection %q", bundleEntry.irodsPath)
					err := manager.filesystem.RemoveDir(bundleEntry.irodsPath, true, true)
					if err != nil {
						manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
						return errors.Wrapf(err, "failed to delete existing collection %q", bundleEntry.irodsPath)
					}
				}
			} else {
				// file
				logger.Debugf("deleting exising data object %q", bundleEntry.irodsPath)

				err := manager.filesystem.RemoveFile(bundleEntry.irodsPath, true)
				if err != nil {
					manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
					return errors.Wrapf(err, "failed to delete existing data object %q", bundleEntry.irodsPath)
				}
			}
		}

		processedFiles++
		manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, false)
	}

	logger.Debugf("deleted exising data objects in the bundle %d", bundle.index)
	return nil
}

func (manager *BundleTransferManager) processBundleTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleTar",
	})

	if len(bundle.entries) == 0 {
		logger.Debugf("skip creating a tarball for bundle %d, empty bundle", bundle.index)
		return nil
	}

	logger.Debugf("creating a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameTar)

	totalFileNum := int64(len(bundle.entries))

	callbackTar := func(processed int64, total int64) {
		manager.progress(progressName, processed, total, progress.UnitsDefault, false)
	}

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	if !bundle.RequireTar() {
		// no tar, so pass this step
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		logger.Debugf("skip creating a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)
		return nil
	}

	entries := make([]string, len(bundle.entries))
	for idx, entry := range bundle.entries {
		entries[idx] = entry.localPath
	}

	err := Tar(manager.localBundleRootPath, entries, bundle.localBundlePath, callbackTar)
	if err != nil {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
		return errors.Wrapf(err, "failed to create a tarball for bundle %d to %q (bundle root %q)", bundle.index, bundle.localBundlePath, bundle.manager.localBundleRootPath)
	}

	manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)

	logger.Debugf("created a tarball for bundle %d to %q", bundle.index, bundle.localBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleUpload(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleUpload",
	})

	if len(bundle.entries) == 0 {
		logger.Debugf("skip uploading bundle %d, empty bundle", bundle.index)
		return nil
	}

	logger.Debugf("uploading bundle %d to %q", bundle.index, bundle.irodsBundlePath)

	if bundle.RequireTar() {
		return manager.processBundleUploadWithTar(bundle)
	}

	return manager.processBundleUploadWithoutTar(bundle)
}

func (manager *BundleTransferManager) processBundleUploadWithTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleUploadWithTar",
	})

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	callbackPut := func(processed int64, total int64) {
		manager.progress(progressName, processed, total, progress.UnitsBytes, false)
	}

	// check local bundle file
	localBundleStat, err := os.Stat(bundle.localBundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
			return irodsclient_types.NewFileNotFoundError(bundle.localBundlePath)
		}

		return errors.Wrapf(err, "failed to stat %q", bundle.localBundlePath)
	}

	// check irods bundle file of previous run
	bundleEntry, err := manager.filesystem.StatFile(bundle.irodsBundlePath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
			return errors.Wrapf(err, "failed to stat existing bundle %q", bundle.irodsBundlePath)
		}
	} else {
		if bundleEntry.Size == localBundleStat.Size() {
			// same file exist
			manager.progress(progressName, bundle.size, bundle.size, progress.UnitsBytes, false)
			// remove local bundle file
			os.Remove(bundle.localBundlePath)
			logger.Debugf("skip uploading bundle %d to %q, file already exists", bundle.index, bundle.irodsBundlePath)
			return nil
		}
	}

	logger.Debugf("uploading bundle %d to %q, size %d", bundle.index, bundle.irodsBundlePath, localBundleStat.Size())

	notes := []string{}

	// determine how to upload
	startTime := time.Now()
	transferMode := manager.determineTransferMode(bundle.size)
	threadsRequired := bundle.GetThreadsRequired()
	_, err = manager.filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", threadsRequired, false, manager.verifyChecksum, manager.verifyChecksum, false, callbackPut)
	notes = append(notes, "icat", "bundle", fmt.Sprintf("%d threads", threadsRequired))

	if err != nil {
		manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
		return errors.Wrapf(err, "failed to upload bundle %d to %q", bundle.index, bundle.irodsBundlePath)
	}

	endTime := time.Now()
	notes = append(notes, fmt.Sprintf("bundle_idx:%d", bundle.index))
	notes = append(notes, fmt.Sprintf("bundle_path:%s", bundle.irodsBundlePath))

	for _, bundleEntry := range bundle.entries {
		uploadResult := irodsclient_fs.FileTransferResult{
			IRODSPath: bundleEntry.irodsPath,
			IRODSSize: bundleEntry.size,
			LocalPath: bundleEntry.localPath,
			LocalSize: bundleEntry.size,
			StartTime: startTime,
			EndTime:   endTime,
		}

		err = manager.transferReportManager.AddTransfer(&uploadResult, TransferMethodBput, err, notes)
		if err != nil {
			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
			return errors.Wrapf(err, "failed to add transfer report")
		}
	}

	// remove local bundle file
	os.Remove(bundle.localBundlePath)

	logger.Debugf("uploaded bundle %d to %q", bundle.index, bundle.irodsBundlePath)

	return nil
}

func (manager *BundleTransferManager) processBundleUploadWithoutTar(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleUploadWithoutTar",
	})

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	fileProgress := make([]int64, len(bundle.entries))

	manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, false)

	for fileIdx, file := range bundle.entries {
		callbackPut := func(processed int64, total int64) {
			fileProgress[fileIdx] = processed

			progressSum := int64(0)
			for _, progress := range fileProgress {
				progressSum += progress
			}

			manager.progress(progressName, progressSum, bundle.size, progress.UnitsBytes, false)
		}

		if file.dir {
			// make dir
			err := manager.filesystem.MakeDir(file.irodsPath, true)
			if err != nil {
				manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
				return errors.Wrapf(err, "failed to upload a directory %q in bundle %d to %q", file.localPath, bundle.index, file.irodsPath)
			}

			now := time.Now()
			reportFile := &TransferReportFile{
				Method:     TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: file.localPath,
				DestPath:   file.irodsPath,
				Notes:      []string{"directory"},
			}

			manager.transferReportManager.AddFile(reportFile)

			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, false)
			logger.Debugf("uploaded a directory %q in bundle %d to %q", file.localPath, bundle.index, file.irodsPath)
			continue
		}

		// file
		parentDir := path.Dir(file.irodsPath)
		if !manager.filesystem.ExistsDir(parentDir) {
			// if parent dir does not exist, create
			err := manager.filesystem.MakeDir(parentDir, true)
			if err != nil {
				manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
				return errors.Wrapf(err, "failed to make a collection %q to upload file %q in bundle %d to %q", parentDir, file.localPath, bundle.index, file.irodsPath)
			}
		}

		notes := []string{}

		// determine how to upload
		transferMode := manager.determineTransferMode(bundle.size)
		threadsRequired := manager.calculateThreadForFileTransfer(file.size)

		uploadResult, err := manager.filesystem.UploadFileParallel(file.localPath, file.irodsPath, "", threadsRequired, false, true, true, false, callbackPut)
		notes = append(notes, "icat", "no-bundle", fmt.Sprintf("%d threads", threadsRequired))

		if err != nil {
			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
			return errors.Wrapf(err, "failed to upload file %q in bundle %d to %q", file.localPath, bundle.index, file.irodsPath)
		}

		notes = append(notes, fmt.Sprintf("bundle_idx:%d", bundle.index))

		err = manager.transferReportManager.AddTransfer(uploadResult, TransferMethodBput, err, notes)
		if err != nil {
			manager.progress(progressName, 0, bundle.size, progress.UnitsBytes, true)
			return errors.Wrapf(err, "failed to add transfer report")
		}

		manager.progress(progressName, file.size, bundle.size, progress.UnitsBytes, false)
		logger.Debugf("uploaded file %q in bundle %d to %q", file.localPath, bundle.index, file.irodsPath)
	}

	logger.Debugf("uploaded files in bundle %d to %q", bundle.index, bundle.irodsBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleExtract(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_transfer",
		"struct":   "BundleTransferManager",
		"function": "processBundleExtract",
	})

	if len(bundle.entries) == 0 {
		logger.Debugf("skip extracting bundle %d, empty bundle", bundle.index)
		return nil
	}

	logger.Debugf("extracting bundle %d at %q", bundle.index, bundle.irodsBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameExtract)

	totalFileNum := int64(len(bundle.entries))

	manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)

	if bundle.RequireTar() {
		err := manager.filesystem.ExtractStructFile(bundle.irodsBundlePath, manager.irodsDestPath, "", irodsclient_types.TAR_FILE_DT, true, !manager.noBulkRegistration)
		if err != nil {
			manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
			return errors.Wrapf(err, "failed to extract bundle %d at %q to %q", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath)
		}

		// remove irods bundle file
		logger.Debugf("removing bundle %d at %q", bundle.index, bundle.irodsBundlePath)
		manager.filesystem.RemoveFile(bundle.irodsBundlePath, true)
	} else {
		// no tar, so pass this step
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		logger.Debugf("skip extracting bundle %d at %q", bundle.index, bundle.irodsBundlePath)
	}

	manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)

	now := time.Now()

	for _, file := range bundle.entries {
		reportFile := &TransferReportFile{
			Method:     TransferMethodBput,
			StartAt:    now,
			EndAt:      now,
			SourcePath: file.localPath,
			SourceSize: file.size,

			DestPath: file.irodsPath,
			DestSize: file.size,
			Notes:    []string{"bundle_extracted"},
		}

		manager.transferReportManager.AddFile(reportFile)
	}

	logger.Debugf("extracted bundle %d at %q to %q", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath)
	return nil
}

func (manager *BundleTransferManager) getProgressName(bundle *Bundle, taskName string) string {
	return fmt.Sprintf("bundle %d - %q", bundle.index, taskName)
}

func (manager *BundleTransferManager) determineTransferMethodForBundle(bundle *Bundle) (TransferMode, int) {
	if bundle.RequireTar() {
		return manager.determineTransferMethod(bundle.size)
	}

	maxSize := int64(0)
	for _, entry := range bundle.entries {
		if entry.size > maxSize {
			maxSize = entry.size
		}
	}

	return manager.determineTransferMethod(maxSize)
}

func (manager *BundleTransferManager) determineTransferMethod(size int64) (TransferMode, int) {
	threads := CalculateThreadForTransferJob(size, manager.maxUploadThreadsPerFile)

	// determine how to upload
	if manager.maxTotalUploadThreads == 1 || !manager.filesystem.SupportParallelUpload() {
		threads = 1
	}

	if manager.icat {
		return TransferModeICAT, threads
	}

	// sysconfig
	systemConfig := GetSystemConfig()
	if systemConfig != nil && systemConfig.AdditionalConfig != nil {
		if systemConfig.AdditionalConfig.TransferMode.Valid() {
			return systemConfig.AdditionalConfig.TransferMode, threads
		}
	}

	return TransferModeICAT, threads
}

func (manager *BundleTransferManager) determineTransferMode(size int64) TransferMode {
	if manager.icat {
		return TransferModeICAT
	}

	// sysconfig
	systemConfig := GetSystemConfig()
	if systemConfig != nil && systemConfig.AdditionalConfig != nil {
		if systemConfig.AdditionalConfig.TransferMode.Valid() {
			return systemConfig.AdditionalConfig.TransferMode
		}
	}

	return TransferModeICAT
}
*/

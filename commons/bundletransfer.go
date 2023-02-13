package commons

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
)

// default values
const (
	MaxBundleFileNumDefault  int   = 50
	MaxBundleFileSizeDefault int64 = 1 * 1024 * 1024 * 1024 // 1GB
	MinBundleFileNumDefault  int   = 5
)

const (
	BundleTaskNameRemove  string = "Remove Old Files"
	BundleTaskNameTar     string = "TAR"
	BundleTaskNameUpload  string = "Upload"
	BundleTaskNameExtract string = "Extract"
)

type BundleFile struct {
	LocalPath        string
	IRODSPath        string
	Size             int64
	Hash             string
	LastModifiedTime time.Time
}

type Bundle struct {
	manager *BundleTransferManager

	index             int64
	files             []*BundleFile
	size              int64
	localBundlePath   string
	irodsBundlePath   string
	lastError         error
	lastErrorTaskName string
}

func newBundle(manager *BundleTransferManager, index int64) *Bundle {
	return &Bundle{
		manager:           manager,
		index:             index,
		files:             []*BundleFile{},
		size:              0,
		localBundlePath:   manager.getLocalBundleFilePath(index),
		irodsBundlePath:   manager.getIrodsBundleFilePath(index),
		lastError:         nil,
		lastErrorTaskName: "",
	}
}

func (bundle *Bundle) addFile(localPath string, size int64, hash string, lastModTime time.Time) error {
	irodsPath, err := bundle.manager.getTargetPath(localPath)
	if err != nil {
		return err
	}

	f := &BundleFile{
		LocalPath:        localPath,
		IRODSPath:        irodsPath,
		Size:             size,
		Hash:             hash,
		LastModifiedTime: lastModTime,
	}

	bundle.files = append(bundle.files, f)
	bundle.size += size

	return nil
}

func (bundle *Bundle) isFull() bool {
	return bundle.size >= bundle.manager.maxBundleFileSize || len(bundle.files) >= bundle.manager.maxBundleFileNum
}

func (bundle *Bundle) requireTar() bool {
	return len(bundle.files) >= MinBundleFileNumDefault
}

type BundleTransferManager struct {
	filesystem              *irodsclient_fs.FileSystem
	irodsDestPath           string
	currentBundle           *Bundle
	nextBundleIndex         int64
	pendingBundles          chan *Bundle
	bundleRootPath          string
	maxBundleFileNum        int
	maxBundleFileSize       int64
	localTempDirPath        string
	irodsTempDirPath        string
	differentFilesOnly      bool
	noHashForComparison     bool
	showProgress            bool
	progressWriter          progress.Writer
	progressTrackers        map[string]*progress.Tracker
	progressTrackerCallback ProgressTrackerCallback
	lastError               error
	mutex                   sync.RWMutex

	scheduleWait sync.WaitGroup
	transferWait sync.WaitGroup
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(fs *irodsclient_fs.FileSystem, irodsDestPath string, maxBundleFileNum int, maxBundleFileSize int64, localTempDirPath string, irodsTempDirPath string, diff bool, noHash bool, showProgress bool) *BundleTransferManager {
	manager := &BundleTransferManager{
		filesystem:              fs,
		irodsDestPath:           irodsDestPath,
		currentBundle:           nil,
		nextBundleIndex:         0,
		pendingBundles:          make(chan *Bundle, 100),
		bundleRootPath:          "/",
		maxBundleFileNum:        maxBundleFileNum,
		maxBundleFileSize:       maxBundleFileSize,
		localTempDirPath:        localTempDirPath,
		irodsTempDirPath:        irodsTempDirPath,
		differentFilesOnly:      diff,
		noHashForComparison:     noHash,
		showProgress:            showProgress,
		progressWriter:          nil,
		progressTrackers:        map[string]*progress.Tracker{},
		progressTrackerCallback: nil,
		lastError:               nil,
		mutex:                   sync.RWMutex{},
		scheduleWait:            sync.WaitGroup{},
		transferWait:            sync.WaitGroup{},
	}

	manager.scheduleWait.Add(1)

	return manager
}

func (manager *BundleTransferManager) getNextBundleIndex() int64 {
	idx := manager.nextBundleIndex
	manager.nextBundleIndex++
	return idx
}

func (manager *BundleTransferManager) getBundleFileName(index int64) string {
	return fmt.Sprintf("bundle_%d.tar", index)
}

func (manager *BundleTransferManager) getLocalBundleFilePath(index int64) string {
	return filepath.Join(manager.localTempDirPath, manager.getBundleFileName(index))
}

func (manager *BundleTransferManager) getIrodsBundleFilePath(index int64) string {
	return filepath.Join(manager.irodsTempDirPath, manager.getBundleFileName(index))
}

func (manager *BundleTransferManager) progress(name string, processed int64, total int64, progressUnit progress.Units, errored bool) {
	if manager.progressTrackerCallback != nil {
		manager.progressTrackerCallback(name, processed, total, progressUnit, errored)
	}
}

func (manager *BundleTransferManager) getTargetPath(localPath string) (string, error) {
	relPath, err := filepath.Rel(manager.bundleRootPath, localPath)
	if err != nil {
		return "", err
	}

	return path.Join(manager.irodsDestPath, filepath.ToSlash(relPath)), nil
}

func (manager *BundleTransferManager) Schedule(source string, size int64, lastModTime time.Time) error {
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

			manager.mutex.Lock()
			manager.currentBundle = nil
			manager.transferWait.Add(1)
		}
	}

	if manager.currentBundle == nil {
		// add new
		manager.currentBundle = newBundle(manager, manager.getNextBundleIndex())
		logger.Debugf("assigned a new bundle %d", manager.currentBundle.index)
	}

	if manager.differentFilesOnly {
		targetFilePath, err := manager.getTargetPath(source)
		if err != nil {
			return err
		}

		exist := ExistsIRODSFile(manager.filesystem, targetFilePath)
		if exist {
			targetEntry, err := StatIRODSPath(manager.filesystem, targetFilePath)
			if err != nil {
				return err
			}

			if manager.noHashForComparison {
				if targetEntry.Size == size {
					fmt.Printf("skip adding a file %s to the bundle. The file already exists!\n", targetFilePath)
					return nil
				}
			} else {
				if targetEntry.Size == size {
					if len(targetEntry.CheckSum) > 0 {
						// compare hash
						md5hash, err := HashLocalFileMD5(source)
						if err != nil {
							return err
						}

						if md5hash == targetEntry.CheckSum {
							fmt.Printf("skip adding a file %s to the bundle. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}
			}
		}
	}

	manager.currentBundle.addFile(source, size, "", lastModTime)

	manager.mutex.Unlock()
	return nil
}

func (manager *BundleTransferManager) DoneScheduling() {
	manager.mutex.Lock()
	if manager.currentBundle != nil {
		manager.pendingBundles <- manager.currentBundle
		manager.currentBundle = nil
		manager.transferWait.Add(1)
	}
	manager.mutex.Unlock()

	close(manager.pendingBundles)
	manager.scheduleWait.Done()
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

	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return manager.lastError
}

func (manager *BundleTransferManager) SetBundleRootPath(bundleRootPath string) {
	manager.bundleRootPath = bundleRootPath
}

func (manager *BundleTransferManager) startProgress() {
	if manager.showProgress {
		manager.progressWriter = progress.NewWriter()
		manager.progressWriter.SetAutoStop(false)
		manager.progressWriter.SetTrackerLength(25)
		manager.progressWriter.SetMessageWidth(50)
		manager.progressWriter.SetStyle(progress.StyleDefault)
		manager.progressWriter.SetTrackerPosition(progress.PositionRight)
		manager.progressWriter.SetUpdateFrequency(time.Millisecond * 100)
		manager.progressWriter.Style().Colors = progress.StyleColorsExample
		manager.progressWriter.Style().Options.PercentFormat = "%4.1f%%"
		manager.progressWriter.Style().Visibility.ETA = true
		manager.progressWriter.Style().Visibility.Percentage = true
		manager.progressWriter.Style().Visibility.Time = true
		manager.progressWriter.Style().Visibility.Value = true
		manager.progressWriter.Style().Visibility.ETAOverall = false
		manager.progressWriter.Style().Visibility.TrackerOverall = false

		go manager.progressWriter.Render()

		// add progress tracker callback
		manager.progressTrackerCallback = func(name string, processed int64, total int64, progressUnit progress.Units, errored bool) {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			var tracker *progress.Tracker
			if t, ok := manager.progressTrackers[name]; !ok {
				// created a new tracker if not exists
				tracker = &progress.Tracker{
					Message: name,
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
	processBundleRemoveFilesChan := make(chan *Bundle, 5)
	processBundleUploadChan := make(chan *Bundle, 5)
	processBundleExtractChan1 := make(chan *Bundle, 5)
	processBundleExtractChan2 := make(chan *Bundle, 5)

	manager.startProgress()

	// bundle --> tar --> upload   --> extract
	//        --> remove ------------>

	go func() {
		logger.Debug("start transfer thread 1")
		defer logger.Debug("exit transfer thread 1")

		defer close(processBundleTarChan)
		defer close(processBundleRemoveFilesChan)

		err := manager.filesystem.MakeDir(manager.irodsDestPath, true)
		if err != nil {
			// mark error
			manager.mutex.Lock()
			manager.lastError = err
			manager.mutex.Unlock()

			logger.Error(err)
			// don't stop here
		}

		ClearIRODSDirCache(manager.filesystem, manager.irodsDestPath)

		for bundle := range manager.pendingBundles {
			// send to tar and remove
			processBundleTarChan <- bundle
			processBundleRemoveFilesChan <- bundle
			// don't stop here
		}
	}()

	// process bundle - tar
	go func() {
		logger.Debug("start transfer thread 2")
		defer logger.Debug("exit transfer thread 2")

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
				}
			}

			processBundleUploadChan <- bundle
		}
	}()

	// process bundle - upload
	funcAsyncUpload := func(id int, wg *sync.WaitGroup) {
		logger.Debugf("start transfer thread 3-%d", id)
		defer logger.Debugf("exit transfer thread 3-%d", id)

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

				if cont {
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
	for i := 0; i < 5; i++ {
		waitAsyncUpload.Add(1)
		go funcAsyncUpload(i, &waitAsyncUpload)
	}

	go func() {
		waitAsyncUpload.Wait()
		close(processBundleExtractChan1)
	}()

	// process bundle - remove files
	go func() {
		logger.Debug("start transfer thread 4")
		defer logger.Debug("exit transfer thread 4")

		defer close(processBundleExtractChan2)

		for bundle := range processBundleRemoveFilesChan {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont {
				err := manager.processBundleRemoveFiles(bundle)
				if err != nil {
					// mark error
					manager.mutex.Lock()
					manager.lastError = err
					manager.mutex.Unlock()

					bundle.lastError = err
					bundle.lastErrorTaskName = BundleTaskNameRemove

					logger.Error(err)
					// don't stop here
				}
			}

			processBundleExtractChan2 <- bundle
		}
	}()

	// process bundle - extract
	go func() {
		logger.Debug("start transfer thread 5")
		defer logger.Debug("exit transfer thread 5")

		defer manager.endProgress()

		// order may be different
		removeTaskCompleted := map[int64]int{}
		removeTaskCompletedMutex := sync.Mutex{}

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

						if cont {
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

						defer manager.transferWait.Done()
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

						if cont {
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

						defer manager.transferWait.Done()
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
	}()
}

func (manager *BundleTransferManager) processBundleRemoveFiles(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleRemoveFiles",
	})

	// remove files in the bundle if they exist in iRODS
	logger.Debugf("deleting exising data objects in the bundle %d", bundle.index)

	progressName := manager.getProgressName(bundle, BundleTaskNameRemove)

	totalFileNum := int64(len(bundle.files))
	processedFiles := int64(0)

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)
	}

	if !bundle.requireTar() {
		// no tar, we can overwrite it. so pass this step
		if manager.showProgress {
			manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		}

		logger.Debugf("skip - deleting exising data objects in the bundle %d, we will overwrite them", bundle.index)
		return nil
	}

	for _, file := range bundle.files {
		if ExistsIRODSFile(manager.filesystem, file.IRODSPath) {
			logger.Debugf("deleting exising data object %s", file.IRODSPath)

			err := manager.filesystem.RemoveFile(file.IRODSPath, true)
			if err != nil {
				if manager.showProgress {
					manager.progress(progressName, processedFiles, totalFileNum, progress.UnitsDefault, true)
				}

				logger.Error(err)
				return fmt.Errorf("failed to delete existing data object %s", file.IRODSPath)
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

	logger.Debugf("creating a tarball for bundle %d to %s", bundle.index, bundle.localBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameTar)

	totalFileNum := int64(len(bundle.files))

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

		logger.Debugf("skip - creating a tarball for bundle %d to %s", bundle.index, bundle.localBundlePath)
		return nil
	}

	files := make([]string, len(bundle.files))
	for idx, file := range bundle.files {
		files[idx] = file.LocalPath
	}

	err := Tar(manager.bundleRootPath, files, bundle.localBundlePath, callback)
	if err != nil {
		if manager.showProgress {
			manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, true)
		}

		logger.WithError(err).Errorf("failed to create a tarball for bundle %d to %s", bundle.index, bundle.localBundlePath)
		return err
	}

	logger.Debugf("created a tarball for bundle %d to %s", bundle.index, bundle.localBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleUpload(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleUpload",
	})

	logger.Debugf("uploading bundle %d to %s", bundle.index, bundle.irodsBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameUpload)

	config := GetConfig()
	totalFileSize := bundle.size
	replicate := !config.NoReplication

	if bundle.requireTar() {
		var callback func(processed int64, total int64)
		if manager.showProgress {
			callback = func(processed int64, total int64) {
				manager.progress(progressName, processed, total, progress.UnitsBytes, false)
			}
		}

		err := manager.filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, callback)
		if err != nil {
			if manager.showProgress {
				manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
			}

			logger.WithError(err).Errorf("failed to upload bundle %d to %s", bundle.index, bundle.irodsBundlePath)
			return err
		}

		// remove local bundle file
		os.Remove(bundle.localBundlePath)
	} else {
		fileUploadProgress := make([]int64, len(bundle.files))
		fileUploadProgressMutex := sync.Mutex{}

		if manager.showProgress {
			manager.progress(progressName, 0, totalFileSize, progress.UnitsBytes, false)
		}

		var asyncErr error
		wg := sync.WaitGroup{}

		for fileIdx := range bundle.files {
			// this is for safe access to file in the bundle array
			file := bundle.files[fileIdx]
			wg.Add(1)

			go func(progressIdx int) {
				defer wg.Done()

				var callbackFileUpload func(processed int64, total int64)
				if manager.showProgress {
					callbackFileUpload = func(processed int64, total int64) {
						fileUploadProgressMutex.Lock()
						defer fileUploadProgressMutex.Unlock()

						fileUploadProgress[progressIdx] = processed

						progressSum := int64(0)
						for _, progress := range fileUploadProgress {
							progressSum += progress
						}

						manager.progress(progressName, progressSum, totalFileSize, progress.UnitsBytes, false)
					}
				}

				if !ExistsIRODSDir(manager.filesystem, path.Dir(file.IRODSPath)) {
					// if parent dir does not exist, create
					err := manager.filesystem.MakeDir(path.Dir(file.IRODSPath), true)
					if err != nil {
						if manager.showProgress {
							manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
						}

						logger.WithError(err).Errorf("failed to create a dir %s to upload file %s in bundle %d to %s", path.Dir(file.IRODSPath), file.LocalPath, bundle.index, file.IRODSPath)
						asyncErr = err
						//return err
						return
					}

					ClearIRODSDirCache(manager.filesystem, path.Dir(file.IRODSPath))
				}

				err := manager.filesystem.UploadFileParallel(file.LocalPath, file.IRODSPath, "", 0, replicate, callbackFileUpload)
				if err != nil {
					if manager.showProgress {
						manager.progress(progressName, -1, totalFileSize, progress.UnitsBytes, true)
					}

					logger.WithError(err).Errorf("failed to upload file %s in bundle %d to %s", file.LocalPath, bundle.index, file.IRODSPath)
					asyncErr = err
					//return err
					return
				}

				logger.Debugf("uploaded file %s in bundle %d to %s", file.LocalPath, bundle.index, file.IRODSPath)
			}(fileIdx)
		}

		wg.Wait()

		if asyncErr != nil {
			return asyncErr
		}
	}

	logger.Debugf("uploaded bundle %d to %s", bundle.index, bundle.irodsBundlePath)
	return nil
}

func (manager *BundleTransferManager) processBundleExtract(bundle *Bundle) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "processBundleExtract",
	})

	logger.Debugf("extracting bundle %d at %s", bundle.index, bundle.irodsBundlePath)

	progressName := manager.getProgressName(bundle, BundleTaskNameExtract)

	totalFileNum := int64(len(bundle.files))

	if manager.showProgress {
		manager.progress(progressName, 0, totalFileNum, progress.UnitsDefault, false)
	}

	if !bundle.requireTar() {
		// no tar, so pass this step
		if manager.showProgress {
			manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
		}

		logger.Debugf("skip - extracting bundle %d at %s", bundle.index, bundle.irodsBundlePath)
		return nil
	}

	err := manager.filesystem.ExtractStructFile(bundle.irodsBundlePath, manager.irodsDestPath, "", types.TAR_FILE_DT, true)
	if err != nil {
		if manager.showProgress {
			manager.progress(progressName, -1, totalFileNum, progress.UnitsDefault, true)
		}

		logger.WithError(err).Errorf("failed to extract bundle %d at %s to %s", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath)

		manager.filesystem.RemoveFile(bundle.irodsBundlePath, true)
		return err
	}

	// remove irods bundle file
	manager.filesystem.RemoveFile(bundle.irodsBundlePath, true)

	if manager.showProgress {
		manager.progress(progressName, totalFileNum, totalFileNum, progress.UnitsDefault, false)
	}

	logger.Debugf("extracted bundle %d at %s to %s", bundle.index, bundle.irodsBundlePath, manager.irodsDestPath)
	return nil
}

func (manager *BundleTransferManager) getProgressName(bundle *Bundle, taskName string) string {
	return fmt.Sprintf("bundle %d - %s", bundle.index, taskName)
}

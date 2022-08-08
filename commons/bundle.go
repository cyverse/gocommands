package commons

import (
	"container/list"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
)

const (
	MaxBundleFileNum  int   = 100
	MaxBundleFileSize int64 = 5 * 1024 * 1024 * 1024 // 5GB
)

type Bundle struct {
	name            string
	files           []string
	size            int64
	localBundlePath string
	irodsBundlePath string
}

func NewBundle() *Bundle {
	return &Bundle{
		files:           []string{},
		size:            0,
		localBundlePath: "",
		irodsBundlePath: "",
	}
}

func (bundle *Bundle) AddFile(path string, size int64) {
	bundle.files = append(bundle.files, path)
	bundle.size += size
}

func (bundle *Bundle) Seal(name string) error {
	// id
	strs := []string{}
	strs = append(strs, bundle.files...)
	strs = append(strs, fmt.Sprintf("%d", bundle.size))

	bundleID, err := HashStringsMD5(strs)
	if err != nil {
		return err
	}

	// name
	bundle.name = name

	// set local bundle path
	tempDir := os.TempDir()
	bundle.localBundlePath = filepath.Join(tempDir, fmt.Sprintf("%s_%s.tar", bundle.name, bundleID))
	return nil
}

type BundleTransferManager struct {
	pendingBundles          *list.List // *Bundle
	maxBundleFileNum        int
	maxBundleFileSize       int64
	errors                  *list.List // error
	progressTrackerCallback ProgressTrackerCallback
	mutex                   sync.Mutex
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(maxBundleFileNum int, maxBundleFileSize int64) *BundleTransferManager {
	manager := &BundleTransferManager{
		pendingBundles:          list.New(),
		maxBundleFileNum:        maxBundleFileNum,
		maxBundleFileSize:       maxBundleFileSize,
		errors:                  list.New(),
		progressTrackerCallback: nil,
	}

	return manager
}

func (manager *BundleTransferManager) progressCallback(name string, processed int64, total int64) {
	if manager.progressTrackerCallback != nil {
		manager.progressTrackerCallback(name, processed, total)
	}
}

// ScheduleBundleUpload schedules a file bundle upload
func (manager *BundleTransferManager) ScheduleBundleUpload(source string, size int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "ScheduleBundleUpload",
	})

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	lastElem := manager.pendingBundles.Back()
	var currentBundle *Bundle
	if lastElem != nil {
		if lastBundle, ok := lastElem.Value.(*Bundle); ok {
			if lastBundle.size >= manager.maxBundleFileSize || len(lastBundle.files) >= manager.maxBundleFileNum {
				// exceed bundle size or file num
				// create a new
				currentBundle = NewBundle()

				logger.Debugf("assigning a new bundle %d", manager.pendingBundles.Len())
				manager.pendingBundles.PushBack(currentBundle)
			} else {
				// safe to add
				currentBundle = lastBundle
			}
		} else {
			return fmt.Errorf("cannot convert the last element in pending bundles to Bundle type")
		}
	} else {
		// add new
		currentBundle = NewBundle()

		logger.Debugf("assigning a new bundle %d", manager.pendingBundles.Len())
		manager.pendingBundles.PushBack(currentBundle)
	}

	currentBundle.AddFile(source, size)

	return nil
}

// run jobs
func (manager *BundleTransferManager) Go(filesystem *irodsclient_fs.FileSystem, tempPath string, targetPath string, force bool, showProgress bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "BundleTransferManager",
		"function": "Go",
	})

	manager.mutex.Lock()
	pendingBundles := manager.pendingBundles.Len()
	manager.mutex.Unlock()

	if pendingBundles == 0 {
		logger.Debug("no pending bundles found")
		return nil
	}

	// do some precheck
	bundles := list.New()
	sourceFiles := []string{}

	manager.mutex.Lock()

	for bundleId := 0; bundleId < pendingBundles; bundleId++ {
		bundleElem := manager.pendingBundles.Front()
		if bundleElem != nil {
			bundlePtr := manager.pendingBundles.Remove(bundleElem)
			if bundle, ok := bundlePtr.(*Bundle); ok {
				// seal
				bundleName := fmt.Sprintf("bundle_%d", bundleId)
				logger.Debugf("sealing a bundle '%s'", bundleName)
				bundle.Seal(bundleName)

				bundles.PushBack(bundle)
				sourceFiles = append(sourceFiles, bundle.files...)
			}
		}
	}

	manager.mutex.Unlock()

	// calculate tar root
	tarBaseDir, err := GetCommonRootLocalDirPath(sourceFiles)
	if err != nil {
		logger.Debug("failed to calculate common root path")
		return nil
	}

	logger.Debugf("using %s as tar base dir", tarBaseDir)

	// check if files exist on the target -- to support 'force' option
	// even with 'force' option, ibun fails if files exist
	if force {
		for _, source := range sourceFiles {
			rel, err := filepath.Rel(tarBaseDir, source)
			if err != nil {
				logger.Debugf("failed to calculate relative path for source %s", source)
				return nil
			}

			targetFilepath := path.Join(targetPath, filepath.ToSlash(rel))

			if filesystem.ExistsFile(targetFilepath) {
				logger.Debugf("deleting exising data object %s", targetFilepath)
				err = filesystem.RemoveFile(targetFilepath, true)
				if err != nil {
					logger.Debugf("failed to delete existing data object %s", targetFilepath)
					return nil
				}
			}
		}
	}

	wg := sync.WaitGroup{}

	wg.Add(1)

	var pw progress.Writer
	if showProgress {
		pw = progress.NewWriter()
		pw.SetAutoStop(false)
		pw.SetTrackerLength(25)
		pw.SetMessageWidth(50)
		pw.SetNumTrackersExpected(pendingBundles * 3)
		pw.SetStyle(progress.StyleDefault)
		pw.SetTrackerPosition(progress.PositionRight)
		pw.SetUpdateFrequency(time.Millisecond * 100)
		pw.Style().Colors = progress.StyleColorsExample
		pw.Style().Options.PercentFormat = "%4.1f%%"
		pw.Style().Visibility.ETA = true
		pw.Style().Visibility.Percentage = true
		pw.Style().Visibility.Time = true
		pw.Style().Visibility.Value = true
		pw.Style().Visibility.ETAOverall = true
		pw.Style().Visibility.TrackerOverall = true

		go pw.Render()

		trackers := map[string]*progress.Tracker{}
		trackerMutex := sync.Mutex{}

		// add progress tracker callback
		trackerCB := func(name string, processed int64, total int64) {
			trackerMutex.Lock()
			defer trackerMutex.Unlock()

			var tracker *progress.Tracker
			unit := progress.UnitsDefault
			if manager.isProgressNameForUpload(name) {
				unit = progress.UnitsBytes
			}

			if t, ok := trackers[name]; !ok {
				// not created yet
				tracker = &progress.Tracker{
					Message: name,
					Total:   total,
					Units:   unit,
				}

				pw.AppendTracker(tracker)
				trackers[name] = tracker
			} else {
				tracker = t
			}

			tracker.SetValue(processed)

			if processed >= total {
				tracker.MarkAsDone()
			}
		}

		manager.progressTrackerCallback = trackerCB
	}

	// extractChannel
	extractBundleChan := make(chan *Bundle, 10)

	// tar
	go func() {
		bundlesLen := bundles.Len()
		for i := 0; i < bundlesLen; i++ {
			bundleElem := bundles.Front()
			if bundleElem == nil {
				break
			}

			bundlePtr := bundles.Remove(bundleElem)

			if bundle, ok := bundlePtr.(*Bundle); ok {
				// do bundle
				logger.Debugf("bundling (tar) files to %s", bundle.localBundlePath)

				if showProgress {
					manager.progressCallback(manager.getTarProgressName(bundle), 0, 100)
				}

				err := Tar(tarBaseDir, bundle.files, bundle.localBundlePath)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("failed to bundle (tar) files to %s", bundle.localBundlePath)
					manager.errors.PushBack(err)
					break
				}

				if showProgress {
					manager.progressCallback(manager.getTarProgressName(bundle), 100, 100)
				}

				logger.Debugf("created a bundle (tar) file %s", bundle.localBundlePath)

				// update target irods file path
				bundle.irodsBundlePath = EnsureTargetIRODSFilePath(filesystem, bundle.localBundlePath, tempPath)

				// upload
				logger.Debugf("uploading a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)
				var callback func(processed int64, total int64)
				if showProgress {
					uploadProgressName := manager.getUploadProgressName(bundle)
					callback = func(processed int64, total int64) {
						manager.progressCallback(uploadProgressName, processed, total)
					}
				}

				if filesystem.ExistsFile(bundle.irodsBundlePath) {
					// exists - skip
					logger.Debugf("bundle file already exists in iRODS, skip uploading - %s", bundle.irodsBundlePath)
				} else {
					err = filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false, callback)
					if err != nil {
						manager.mutex.Lock()
						defer manager.mutex.Unlock()

						logger.WithError(err).Errorf("error while uploading a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)
						manager.errors.PushBack(err)
						break
					}

					logger.Debugf("uploaded a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)
				}

				// remove local bundle file
				os.Remove(bundle.localBundlePath)

				extractBundleChan <- bundle
			} else {
				logger.Error("unknown bundle")
			}
		}

		close(extractBundleChan)
	}()

	// extract
	go func() {
		for {
			if bundle, ok := <-extractBundleChan; ok {
				// do extract
				logger.Debugf("extracting a bundle file %s", bundle.irodsBundlePath)
				if showProgress {
					manager.progressCallback(manager.getExtractProgressName(bundle), 0, 100)
				}

				err := filesystem.ExtractStructFile(bundle.irodsBundlePath, targetPath, "", types.TAR_FILE_DT, force)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while extracting a bundle file %s to %s", bundle.irodsBundlePath, targetPath)
					manager.errors.PushBack(err)
					break
				}

				filesystem.RemoveFile(bundle.irodsBundlePath, true)

				if showProgress {
					manager.progressCallback(manager.getExtractProgressName(bundle), 100, 100)
				}

				logger.Debugf("extracted a bundle file %s to %s", bundle.irodsBundlePath, targetPath)
			} else {
				// closed
				break
			}
		}

		wg.Done()
	}()

	wg.Wait()

	if showProgress {
		pw.Stop()
	}

	// check error
	var errReturn error
	manager.mutex.Lock()
	if manager.errors.Len() > 0 {
		frontElem := manager.errors.Front()
		if frontElem != nil {
			if err, ok := frontElem.Value.(error); ok {
				errReturn = err
			}
		}
	}
	manager.mutex.Unlock()
	return errReturn
}

func (manager *BundleTransferManager) getTarProgressName(bundle *Bundle) string {
	return fmt.Sprintf("%s - TAR", bundle.name)
}

func (manager *BundleTransferManager) getUploadProgressName(bundle *Bundle) string {
	return fmt.Sprintf("%s - Upload", bundle.name)
}

func (manager *BundleTransferManager) isProgressNameForUpload(name string) bool {
	return strings.HasSuffix(name, " Upload")
}

func (manager *BundleTransferManager) getExtractProgressName(bundle *Bundle) string {
	return fmt.Sprintf("%s - Extract", bundle.name)
}

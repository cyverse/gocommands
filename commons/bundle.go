package commons

import (
	"container/list"
	"fmt"
	"os"
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

const (
	MaxBundleFileNum  int   = 100
	MaxBundleFileSize int64 = 5 * 1024 * 1024 * 1024 // 5GB
)

type Bundle struct {
	files           []string
	size            int64
	localBundlePath string
	irodsBundlePath string
}

func NewBundle() *Bundle {
	bundleID := xid.New().String()
	tempDir := os.TempDir()

	return &Bundle{
		files:           []string{},
		size:            0,
		localBundlePath: fmt.Sprintf("/%s/%s.tar", tempDir, bundleID),
		irodsBundlePath: "",
	}
}

func (bundle *Bundle) AddFile(path string, size int64) {
	bundle.files = append(bundle.files, path)
	bundle.size += size
}

type BundleTransferManager struct {
	pendingBundles     *list.List   // *Bundle
	transferredBundles chan *Bundle // *Bundle
	maxBundleFileNum   int
	maxBundleFileSize  int64
	errors             *list.List // error
	mutex              sync.Mutex
}

// NewBundleTransferManager creates a new BundleTransferManager
func NewBundleTransferManager(maxBundleFileNum int, maxBundleFileSize int64) *BundleTransferManager {
	manager := &BundleTransferManager{
		pendingBundles:     list.New(),
		transferredBundles: make(chan *Bundle),
		maxBundleFileNum:   maxBundleFileNum,
		maxBundleFileSize:  maxBundleFileSize,
		errors:             list.New(),
	}

	return manager
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
				logger.Debugf("assigning a new bundle - %s", currentBundle.localBundlePath)
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
		logger.Debugf("assigning a new bundle - %s", currentBundle.localBundlePath)
		manager.pendingBundles.PushBack(currentBundle)
	}

	currentBundle.AddFile(source, size)

	return nil
}

// run jobs
func (manager *BundleTransferManager) Go(filesystem *irodsclient_fs.FileSystem, tempPath string, targetPath string, force bool) error {
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

	wg := sync.WaitGroup{}

	wg.Add(1)

	// tar
	go func() {
		for i := 0; i < pendingBundles; i++ {
			manager.mutex.Lock()
			frontElem := manager.pendingBundles.Front()
			if frontElem == nil {
				manager.mutex.Unlock()
				logger.Debug("no more pending bundles")
				break
			}

			frontBundle := manager.pendingBundles.Remove(frontElem)
			manager.mutex.Unlock()

			if bundle, ok := frontBundle.(*Bundle); ok {
				// do bundle
				logger.Debugf("bundling (tar) files to %s", bundle.localBundlePath)

				err := Tar(bundle.files, bundle.localBundlePath)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("failed to bundle (tar) files to %s", bundle.localBundlePath)
					manager.errors.PushBack(err)
					break
				}

				logger.Debugf("created a bundle (tar) file %s", bundle.localBundlePath)

				// update target irods file path
				bundle.irodsBundlePath = EnsureTargetIRODSFilePath(filesystem, bundle.localBundlePath, tempPath)

				// upload
				logger.Debugf("uploading a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)
				err = filesystem.UploadFileParallel(bundle.localBundlePath, bundle.irodsBundlePath, "", 0, false)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while uploading a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)
					manager.errors.PushBack(err)
					break
				}

				os.Remove(bundle.localBundlePath)

				logger.Debugf("uploaded a local bundle file %s to %s", bundle.localBundlePath, bundle.irodsBundlePath)

				manager.transferredBundles <- bundle
			} else {
				logger.Error("unknown bundle")
			}
		}

		close(manager.transferredBundles)
		logger.Debug("done bundling")
	}()

	// extract
	go func() {
		for {
			if bundle, ok := <-manager.transferredBundles; ok {
				// do extract
				logger.Debugf("extracting a bundle file %s", bundle.irodsBundlePath)
				err := filesystem.ExtractStructFile(bundle.irodsBundlePath, targetPath, "", types.TAR_FILE_DT, force)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while extracting a bundle file %s to %s", bundle.irodsBundlePath, targetPath)
					manager.errors.PushBack(err)
					break
				}

				filesystem.RemoveFile(bundle.irodsBundlePath, true)

				logger.Debugf("extracted a bundle file %s to %s", bundle.irodsBundlePath, targetPath)
			} else {
				// closed
				break
			}
		}

		wg.Done()
		logger.Debug("done extracting")
	}()

	wg.Wait()

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

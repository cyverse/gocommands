package commons

import (
	"container/list"
	"os"
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
)

const (
	MaxThreadNum int = 5
)

type ParallelTransferJob struct {
	task    func()
	threads int
}

func NewParallelTransferJob(task func(), threads int) *ParallelTransferJob {
	return &ParallelTransferJob{
		task:    task,
		threads: threads,
	}
}

type ParallelTransferManager struct {
	pendingJobs    *list.List // *ParallelTransferJob
	currentThreads int
	maxThreads     int
	errors         *list.List // error
	mutex          sync.Mutex
	condition      *sync.Cond
}

// NewParallelTransferManager creates a new ParallelTransferManager
func NewParallelTransferManager(maxThreads int) *ParallelTransferManager {
	manager := &ParallelTransferManager{
		pendingJobs:    list.New(),
		currentThreads: 0,
		maxThreads:     maxThreads,
		errors:         list.New(),
	}

	manager.condition = sync.NewCond(&manager.mutex)
	return manager
}

// ScheduleDownloadIfDifferent schedules a file download only if data is changed
func (manager *ParallelTransferManager) ScheduleDownloadIfDifferent(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleDownloadIfDifferent",
	})

	task := func() {
		logger.Debugf("synchronizing a data object %s to %s", source, target)
		sourceEntry, err := filesystem.Stat(source)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while stating a data object %s", source)
			manager.errors.PushBack(err)
			return
		}

		targetStat, err := os.Stat(target)
		if err != nil {
			if !os.IsNotExist(err) {
				manager.mutex.Lock()
				defer manager.mutex.Unlock()

				logger.WithError(err).Errorf("error while stating a local file %s", target)
				manager.errors.PushBack(err)
				return
			}

			logger.Debugf("there is no file %s at local", target)
		} else {
			// file/dir exists
			if targetStat.IsDir() {
				// dir
				logger.Debugf("local path %s is a directory, deleting", target)
				err = os.RemoveAll(target)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a directory %s", target)
					manager.errors.PushBack(err)
					return
				}
			} else {
				//file
				md5hash, err := HashLocalFileMD5(target)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while hasing a local file %s", target)
					manager.errors.PushBack(err)
					return
				}

				if sourceEntry.CheckSum == md5hash && sourceEntry.Size == targetStat.Size() {
					// match
					logger.Debugf("local file %s is up-to-date", target)
					return
				}

				// delete first
				logger.Debugf("local file %s is has a different hash code, deleting", target)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", sourceEntry.CheckSum, md5hash, sourceEntry.Size, targetStat.Size())
				err = os.Remove(target)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a local file %s", target)
					manager.errors.PushBack(err)
					return
				}
			}
		}

		// down
		logger.Debugf("downloading a data object %s to %s", source, target)
		err = filesystem.DownloadFileParallel(source, "", target, 0)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while downloading a data object %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("downloaded a data object %s to %s", source, target)
		logger.Debugf("synchronized a data object %s to %s", source, target)
	}

	threads := 1
	// 4MB is one thread, max 4 threads
	sourceEntry, err := filesystem.StatFile(source)
	if err != nil {
		return err
	}

	if sourceEntry.Size > 4*1024*1024 {
		threads = int(sourceEntry.Size / 4 * 1024 * 1024)
		if threads > 4 {
			threads = 4
		}
	}

	job := NewParallelTransferJob(task, threads)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// ScheduleDownload schedules a file download
func (manager *ParallelTransferManager) ScheduleDownload(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleDownload",
	})

	task := func() {
		logger.Debugf("downloading a data object %s to %s", source, target)
		err := filesystem.DownloadFileParallel(source, "", target, 0)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while downloading a data object %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("downloaded a data object %s to %s", source, target)
	}

	threads := 1
	// 4MB is one thread, max 4 threads
	sourceEntry, err := filesystem.StatFile(source)
	if err != nil {
		return err
	}

	if sourceEntry.Size > 4*1024*1024 {
		threads = int(sourceEntry.Size / 4 * 1024 * 1024)
		if threads > 4 {
			threads = 4
		}
	}

	job := NewParallelTransferJob(task, threads)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// ScheduleUploadIfDifferent schedules a file upload only if data is changed
func (manager *ParallelTransferManager) ScheduleUploadIfDifferent(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleUploadIfDifferent",
	})

	task := func() {
		logger.Debugf("synchronizing a local file %s to %s", source, target)

		sourceStat, err := os.Stat(source)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while stating a local file %s", source)
			manager.errors.PushBack(err)
			return
		}

		targetEntry, err := filesystem.Stat(target)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				manager.mutex.Lock()
				defer manager.mutex.Unlock()

				logger.WithError(err).Errorf("error while stating a data object %s", target)
				manager.errors.PushBack(err)
				return
			}

			logger.Debugf("there is no file %s at remote", target)
		} else {
			// file/dir exists
			if targetEntry.Type == irodsclient_fs.DirectoryEntry {
				// dir
				logger.Debugf("remote path %s is a collection, deleting", target)
				err = filesystem.RemoveDir(target, true, true)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a collection %s", target)
					manager.errors.PushBack(err)
					return
				}
			} else {
				// file
				md5hash, err := HashLocalFileMD5(source)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while hasing a local file %s", source)
					manager.errors.PushBack(err)
					return
				}

				if targetEntry.CheckSum == md5hash && targetEntry.Size == sourceStat.Size() {
					// match
					logger.Debugf("remote data object %s is up-to-date", target)
					return
				}

				// delete first
				logger.Debugf("data object %s is has a different hash code, deleting", target)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", md5hash, targetEntry.CheckSum, sourceStat.Size(), targetEntry.Size)
				err = filesystem.RemoveFile(target, true)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a data object %s", target)
					manager.errors.PushBack(err)
					return
				}
			}
		}

		// up
		logger.Debugf("uploading a local file %s to %s", source, target)
		err = filesystem.UploadFileParallel(source, "", target, 0, true)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while uploading a local file %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("uploaded a local file %s to %s", source, target)
		logger.Debugf("synchronized a local file %s to %s", source, target)
	}

	threads := 1
	/*
		// DataStore doesn't support the feature yet
			// 4MB is one thread, max 4 threads
			if size > 4*1024*1024 {
				threads = int(size / 4 * 1024 * 1024)
				if threads > 4 {
					threads = 4
				}
			}
	*/

	job := NewParallelTransferJob(task, threads)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// ScheduleUpload schedules a file upload
func (manager *ParallelTransferManager) ScheduleUpload(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleUpload",
	})

	task := func() {
		logger.Debugf("uploading a local file %s to %s", source, target)
		err := filesystem.UploadFileParallel(source, "", target, 0, true)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while uploading a local file %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("uploaded a local file %s to %s", source, target)
	}

	threads := 1
	/*
		// DataStore doesn't support the feature yet
			// 4MB is one thread, max 4 threads
			if size > 4*1024*1024 {
				threads = int(size / 4 * 1024 * 1024)
				if threads > 4 {
					threads = 4
				}
			}
	*/

	job := NewParallelTransferJob(task, threads)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// ScheduleCopy schedules a file copy
func (manager *ParallelTransferManager) ScheduleCopy(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleCopy",
	})

	task := func() {
		logger.Debugf("copying a data object %s to %s", source, target)
		err := filesystem.CopyFileToFile(source, target)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while copying a data object %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("copied a data object %s to %s", source, target)
	}

	job := NewParallelTransferJob(task, 1)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// ScheduleCopyIfDifferent schedules a file copy only if data is changed
func (manager *ParallelTransferManager) ScheduleCopyIfDifferent(filesystem *irodsclient_fs.FileSystem, source string, target string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "ScheduleCopyIfDifferent",
	})

	task := func() {
		logger.Debugf("synchronizing a data object %s to %s", source, target)
		sourceEntry, err := filesystem.Stat(source)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while stating a data object %s", source)
			manager.errors.PushBack(err)
			return
		}

		targetEntry, err := filesystem.Stat(target)
		if err != nil {
			if !os.IsNotExist(err) {
				manager.mutex.Lock()
				defer manager.mutex.Unlock()

				logger.WithError(err).Errorf("error while stating a data object %s", target)
				manager.errors.PushBack(err)
				return
			}
		} else {
			// file/dir exists
			if targetEntry.Type == irodsclient_fs.DirectoryEntry {
				// dir
				logger.Debugf("remote path %s is a collection, deleting", target)
				err = filesystem.RemoveDir(target, true, true)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a directory %s", target)
					manager.errors.PushBack(err)
					return
				}
			} else {
				//file
				if sourceEntry.CheckSum == targetEntry.CheckSum && sourceEntry.Size == targetEntry.Size {
					// match
					logger.Debugf("data object %s is up-to-date", target)
					return
				}

				// delete first
				logger.Debugf("data object %s is has a different hash code, deleting", target)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", sourceEntry.CheckSum, targetEntry.CheckSum, sourceEntry.Size, targetEntry.Size)
				err = filesystem.RemoveFile(target, true)
				if err != nil {
					manager.mutex.Lock()
					defer manager.mutex.Unlock()

					logger.WithError(err).Errorf("error while deleting a data object %s", target)
					manager.errors.PushBack(err)
					return
				}
			}
		}

		// down
		logger.Debugf("copying a data object %s to %s", source, target)
		err = filesystem.CopyFileToFile(source, target)
		if err != nil {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			logger.WithError(err).Errorf("error while copying a data object %s to %s", source, target)
			manager.errors.PushBack(err)
			return
		}
		logger.Debugf("copied a data object %s to %s", source, target)
		logger.Debugf("synchronized a data object %s to %s", source, target)
	}

	job := NewParallelTransferJob(task, 1)

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.pendingJobs.PushBack(job)
	return nil
}

// run jobs
func (manager *ParallelTransferManager) Go() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelTransferManager",
		"function": "Go",
	})

	manager.mutex.Lock()
	pendingJobs := manager.pendingJobs.Len()
	manager.mutex.Unlock()

	if pendingJobs == 0 {
		logger.Debug("no pending jobs found")
		return nil
	}

	wg := sync.WaitGroup{}

	for {
		manager.mutex.Lock()
		frontElem := manager.pendingJobs.Front()
		if frontElem == nil {
			manager.mutex.Unlock()
			logger.Debug("no more pending job")
			break
		}

		frontJob := manager.pendingJobs.Remove(frontElem)
		if job, ok := frontJob.(*ParallelTransferJob); ok {
			for manager.currentThreads+job.threads > manager.maxThreads {
				// wait
				logger.Debugf("waiting for other jobs to complete - current %d, max %d", manager.currentThreads, manager.maxThreads)
				manager.condition.Wait()
			}

			logger.Debugf("# threads : %d, max %d", manager.currentThreads, manager.maxThreads)
			manager.currentThreads += job.threads
			logger.Debugf("# threads : %d, max %d", manager.currentThreads, manager.maxThreads)
			wg.Add(1)

			go func() {
				job.task()

				manager.mutex.Lock()
				manager.currentThreads -= job.threads
				logger.Debugf("# threads : %d, max %d", manager.currentThreads, manager.maxThreads)
				manager.condition.Broadcast()
				manager.mutex.Unlock()
				wg.Done()
			}()
		} else {
			logger.Error("unknown job")
		}

		manager.mutex.Unlock()
	}

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

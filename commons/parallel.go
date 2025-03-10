package commons

import (
	"sync"
	"sync/atomic"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

const (
	minSizePrioritizedTransfer int64 = 1024 * 1024 * 1024
)

type ParallelJobTask func(job *ParallelJob) error

type ParallelJob struct {
	manager *ParallelJobManager

	index           int64
	name            string
	task            ParallelJobTask
	threadsRequired int
	progressUnit    progress.Units

	done bool
}

func (job *ParallelJob) GetManager() *ParallelJobManager {
	return job.manager
}

func (job *ParallelJob) Progress(processed int64, total int64, errored bool) {
	job.manager.progress(job.name, processed, total, job.progressUnit, errored)
}

func (job *ParallelJob) Done() {
	job.done = true
}

func newParallelJob(manager *ParallelJobManager, index int64, name string, task ParallelJobTask, threadsRequired int, progressUnit progress.Units) *ParallelJob {
	return &ParallelJob{
		manager:         manager,
		index:           index,
		name:            name,
		task:            task,
		threadsRequired: threadsRequired,
		progressUnit:    progressUnit,

		done: false,
	}
}

type ParallelJobManager struct {
	// moved to top to avoid 64bit alignment issue
	jobsScheduledCounter int64
	jobsDoneCounter      int64

	filesystem              *irodsclient_fs.FileSystem
	nextJobIndex            int64
	pendingJobs             chan *ParallelJob
	maxThreads              int
	showProgress            bool
	showFullPath            bool
	progressWriter          progress.Writer
	progressTrackers        map[string]*progress.Tracker
	progressTrackerCallback ProgressTrackerCallback
	lastError               error
	mutex                   sync.RWMutex

	availableThreadWaitCondition *sync.Cond // used for checking available threads
	scheduleWait                 sync.WaitGroup
	processWait                  sync.WaitGroup
}

// NewParallelJobManager creates a new ParallelJobManager
func NewParallelJobManager(fs *irodsclient_fs.FileSystem, maxThreads int, showProgress bool, showFullPath bool) *ParallelJobManager {
	manager := &ParallelJobManager{
		filesystem:              fs,
		nextJobIndex:            0,
		pendingJobs:             make(chan *ParallelJob, 100),
		maxThreads:              maxThreads,
		showProgress:            showProgress,
		showFullPath:            showFullPath,
		progressWriter:          nil,
		progressTrackers:        map[string]*progress.Tracker{},
		progressTrackerCallback: nil,
		lastError:               nil,
		mutex:                   sync.RWMutex{},
		scheduleWait:            sync.WaitGroup{},
		processWait:             sync.WaitGroup{},

		jobsScheduledCounter: 0,
		jobsDoneCounter:      0,
	}

	manager.availableThreadWaitCondition = sync.NewCond(&manager.mutex)

	manager.scheduleWait.Add(1)

	return manager
}

func (manager *ParallelJobManager) GetFilesystem() *irodsclient_fs.FileSystem {
	return manager.filesystem
}

func (manager *ParallelJobManager) getNextJobIndex() int64 {
	idx := manager.nextJobIndex
	manager.nextJobIndex++
	return idx
}

func (manager *ParallelJobManager) progress(name string, processed int64, total int64, progressUnit progress.Units, errored bool) {
	if manager.progressTrackerCallback != nil {
		manager.progressTrackerCallback(name, processed, total, progressUnit, errored)
	}
}

func (manager *ParallelJobManager) Schedule(name string, task ParallelJobTask, threadsRequired int, progressUnit progress.Units) error {
	manager.mutex.Lock()

	// do not accept new schedule if there's an error
	if manager.lastError != nil {
		defer manager.mutex.Unlock()
		return manager.lastError
	}

	job := newParallelJob(manager, manager.getNextJobIndex(), name, task, threadsRequired, progressUnit)

	// release lock since adding to chan may block
	manager.mutex.Unlock()

	manager.pendingJobs <- job
	manager.processWait.Add(1)
	atomic.AddInt64(&manager.jobsScheduledCounter, 1)

	return nil
}

func (manager *ParallelJobManager) DoneScheduling() {
	close(manager.pendingJobs)
	manager.scheduleWait.Done()
}

func (manager *ParallelJobManager) Wait() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelJobManager",
		"function": "Wait",
	})

	logger.Debug("waiting schedule-wait")
	manager.scheduleWait.Wait()
	logger.Debug("waiting job-wait")
	manager.processWait.Wait()

	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	if manager.lastError != nil {
		return manager.lastError
	}

	if manager.jobsDoneCounter != manager.jobsScheduledCounter {
		return xerrors.Errorf("jobs '%d/%d' were not completed!", manager.jobsDoneCounter, manager.jobsScheduledCounter)
	}

	return nil
}

func (manager *ParallelJobManager) startProgress() {
	if manager.showProgress {
		manager.progressWriter = GetProgressWriter(true)
		messageWidth := getProgressMessageWidth(true)

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

func (manager *ParallelJobManager) endProgress() {
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

func (manager *ParallelJobManager) Start() {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "ParallelJobManager",
		"function": "Start",
	})

	manager.startProgress()

	go func() {
		logger.Debug("start job run thread")
		defer logger.Debug("exit job run thread")

		defer manager.endProgress()

		currentThreads := 0

		for job := range manager.pendingJobs {
			cont := true

			manager.mutex.RLock()
			if manager.lastError != nil {
				cont = false
			}
			manager.mutex.RUnlock()

			if cont {
				manager.mutex.Lock()
				if currentThreads > 0 {
					for currentThreads+job.threadsRequired > manager.maxThreads {
						// exceeds max threads
						// wait until it becomes available
						logger.Debugf("waiting for other jobs to complete - current %d, max %d", currentThreads, manager.maxThreads)

						manager.availableThreadWaitCondition.Wait()
					}
				}

				currentThreads += job.threadsRequired
				logger.Debugf("# threads : %d, max %d", currentThreads, manager.maxThreads)

				go func(pjob *ParallelJob) {
					defer manager.processWait.Done()

					logger.Debugf("Run job %d, %q", pjob.index, pjob.name)

					err := pjob.task(pjob)

					if err != nil {
						// mark error
						manager.mutex.Lock()
						manager.lastError = err
						manager.mutex.Unlock()

						logger.Error(err)
						// don't stop here
					}

					manager.mutex.Lock()
					currentThreads -= pjob.threadsRequired
					manager.availableThreadWaitCondition.Broadcast()
					manager.mutex.Unlock()

					logger.Debugf("# threads : %d, max %d", currentThreads, manager.maxThreads)

					if pjob.done {
						// increase jobs done counter
						atomic.AddInt64(&manager.jobsDoneCounter, 1)
					}
				}(job)

				manager.mutex.Unlock()
			} else {
				manager.processWait.Done()
			}
		}

		manager.processWait.Wait()
	}()
}

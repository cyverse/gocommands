package parallel

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
)

type ParallelJobTask func(job *ParallelJob) error

type ParallelJob struct {
	manager *ParallelJobManager

	index        int64
	name         string
	task         ParallelJobTask
	weight       int
	progressUnit progress.Units
	canceled     bool
	mutex        sync.Mutex
}

func newParallelJob(manager *ParallelJobManager, name string, task ParallelJobTask, weight int, progressUnit progress.Units) *ParallelJob {
	return &ParallelJob{
		manager:      manager,
		index:        manager.getNextJobIndex(),
		name:         name,
		task:         task,
		weight:       weight,
		progressUnit: progressUnit,
	}
}

func (job *ParallelJob) GetManager() *ParallelJobManager {
	return job.manager
}

func (job *ParallelJob) Progress(taskType string, processed int64, total int64, errored bool) {
	job.manager.progress(taskType, job.name, processed, total, job.progressUnit, errored)
}

func (job *ParallelJob) GetName() string {
	return job.name
}

func (job *ParallelJob) GetWeight() int {
	return job.weight
}

func (job *ParallelJob) SetCanceled() {
	job.mutex.Lock()
	defer job.mutex.Unlock()

	job.canceled = true
}

func (job *ParallelJob) IsCanceled() bool {
	job.mutex.Lock()
	defer job.mutex.Unlock()

	return job.canceled
}

type ParallelJobManager struct {
	// moved to top to avoid 64bit alignment issue
	jobsDoneCounter     int64
	jobsErroredCounter  int64
	jobsCanceledCounter int64

	nextJobIndex            int64
	pendingJobs             *list.List             // list of *ParallelJob
	runningJobs             map[int64]*ParallelJob // map of job index to *ParallelJob
	totalJobs               int
	weightCapacity          int
	currentWeight           int
	showProgress            bool
	showFullPath            bool
	progressWriter          progress.Writer
	progressTrackers        map[string]*progress.Tracker
	progressTrackerCallback terminal.ProgressTrackerCallback
	lastError               error
	canceled                bool // if the job manager is canceled
	mutex                   sync.RWMutex
	waitCond                *sync.Cond // condition variable for waiting on weight capacity

	processWait sync.WaitGroup
}

// NewParallelJobManager creates a new ParallelJobManager
func NewParallelJobManager(weightCapacity int, showProgress bool, showFullPath bool) *ParallelJobManager {
	manager := &ParallelJobManager{
		nextJobIndex:            0,
		pendingJobs:             list.New(),
		runningJobs:             map[int64]*ParallelJob{},
		totalJobs:               0,
		weightCapacity:          weightCapacity,
		currentWeight:           0,
		showProgress:            showProgress,
		showFullPath:            showFullPath,
		progressWriter:          nil,
		progressTrackers:        map[string]*progress.Tracker{},
		progressTrackerCallback: nil,
		lastError:               nil,
		canceled:                false,
		mutex:                   sync.RWMutex{},
		processWait:             sync.WaitGroup{},

		jobsDoneCounter:    0,
		jobsErroredCounter: 0,
	}

	manager.waitCond = sync.NewCond(&manager.mutex)

	return manager
}

func (manager *ParallelJobManager) getNextJobIndex() int64 {
	idx := manager.nextJobIndex
	manager.nextJobIndex++
	return idx
}

func (manager *ParallelJobManager) progress(taskType string, taskName string, processed int64, total int64, progressUnit progress.Units, errored bool) {
	if manager.progressTrackerCallback != nil {
		manager.progressTrackerCallback(taskType, taskName, processed, total, progressUnit, errored)
	}
}

func (manager *ParallelJobManager) waitForWeight(weight int) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	for manager.currentWeight+weight > manager.weightCapacity {
		manager.waitCond.Wait()
	}

	manager.currentWeight += weight
}

func (manager *ParallelJobManager) decWeight(weight int) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.currentWeight -= weight
	manager.waitCond.Broadcast()
}

func (manager *ParallelJobManager) setLastError(err error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.lastError = err

	for _, job := range manager.runningJobs {
		job.SetCanceled()
	}
}

func (manager *ParallelJobManager) GetLastError() error {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	return manager.lastError
}

func (manager *ParallelJobManager) CancelJobs() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.canceled = true
}

func (manager *ParallelJobManager) IsJobCanceled() bool {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	return manager.canceled
}

func (manager *ParallelJobManager) popNextPendingTask() *ParallelJob {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if manager.pendingJobs.Len() == 0 {
		return nil
	}

	elem := manager.pendingJobs.Front()
	manager.pendingJobs.Remove(elem)

	if job, ok := elem.Value.(*ParallelJob); ok {
		manager.runningJobs[job.index] = job
		return job
	}

	return nil
}

func (manager *ParallelJobManager) removeRunningJob(job *ParallelJob) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	delete(manager.runningJobs, job.index)
}

// Schedule schedules a new job to run in parallel
// must be called before Start
func (manager *ParallelJobManager) Schedule(name string, task ParallelJobTask, weight int, progressUnit progress.Units) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	job := newParallelJob(manager, name, task, weight, progressUnit)

	manager.pendingJobs.PushBack(job)
	manager.processWait.Add(1)
	manager.totalJobs++
}

// Start starts the job manager to run the scheduled jobs in parallel
func (manager *ParallelJobManager) Start() error {
	logger := log.WithFields(log.Fields{})

	manager.startProgress()
	defer manager.endProgress()

	for {
		job := manager.popNextPendingTask()
		if job == nil {
			logger.Debug("no more pending jobs, exiting")
			break
		}

		if manager.GetLastError() != nil {
			// mark the job is canceled if there is an error
			job.canceled = true
		}

		if manager.IsJobCanceled() {
			job.canceled = true
		}

		manager.waitForWeight(job.weight)

		logger.Debugf("Run job id %d, name %q, canceled %t", job.index, job.name, job.canceled)

		go func() {
			taskLogger := log.WithFields(log.Fields{
				"job_index": job.index,
				"job_name":  job.name,
				"canceled":  job.canceled,
			})

			err := job.task(job)
			if err != nil {
				// increase jobs errored counter
				atomic.AddInt64(&manager.jobsErroredCounter, 1)

				// mark error
				manager.setLastError(err)
				taskLogger.Error(err)
				// don't stop here
			} else {
				if job.IsCanceled() {
					// increase jobs canceled counter
					atomic.AddInt64(&manager.jobsCanceledCounter, 1)
					taskLogger.Debug("Job canceled")
				} else {
					// increase jobs done counter
					atomic.AddInt64(&manager.jobsDoneCounter, 1)
					taskLogger.Debug("Job completed successfully")
				}
			}

			manager.removeRunningJob(job)
			manager.processWait.Done()

			manager.decWeight(job.weight)
		}()
	}

	logger.Debug("waiting job-wait")
	manager.processWait.Wait()

	err := manager.GetLastError()
	if err != nil {
		return err
	}

	logger.Debugf("all jobs done, total: %d, completed: %d, canceled: %d, errored: %d", manager.totalJobs, manager.jobsDoneCounter, manager.jobsCanceledCounter, manager.jobsErroredCounter)
	return nil
}

func (manager *ParallelJobManager) startProgress() {
	if manager.showProgress {
		manager.progressWriter = terminal.GetProgressWriter(true)
		messageWidth := terminal.GetProgressMessageWidth(true)

		go manager.progressWriter.Render()

		// add progress tracker callback
		manager.progressTrackerCallback = func(taskType string, taskName string, processed int64, total int64, progressUnit progress.Units, errored bool) {
			manager.mutex.Lock()
			defer manager.mutex.Unlock()

			trackerName := terminal.GetTrackerName(taskType, taskName)

			var tracker *progress.Tracker
			if t, ok := manager.progressTrackers[trackerName]; !ok {
				// created a new tracker if not exists
				msg := trackerName
				if !manager.showFullPath {
					msg = terminal.GetShortTrackerMessage(taskType, taskName, messageWidth)
				}

				tracker = &progress.Tracker{
					Message: msg,
					Total:   total,
					Units:   progressUnit,
				}

				manager.progressWriter.AppendTracker(tracker)
				manager.progressTrackers[trackerName] = tracker
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

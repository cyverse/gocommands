package commons

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type FileTransferTask struct {
	LocalPath        string    `json:"local_path"`
	IRODSPath        string    `json:"irods_path"`
	LastModifiedTime time.Time `json:"last_modified_time"`
	Size             int64     `json:"size"`
	Hash             string    `json:"hash"` // md5
	Completed        bool      `json:"completed"`
}

type JobLogHeader struct {
	ID              string    `json:"id"`
	CreatedTime     time.Time `json:"created_time"`
	LocalInputPaths []string  `json:"local_input_paths"`
	IRODSTargetPath string    `json:"irods_target_path"`
}

type JobLog struct {
	id              string
	jobLogFilePath  string
	localInputPaths []string
	irodsTargetPath string
	transferTasks   map[string]*FileTransferTask
	createdTime     time.Time
	writeMutex      sync.Mutex
	isNewJob        bool
}

func NewJobLog(id string, jobLogFilePath string, localInputPaths []string, irodsTargetPath string) *JobLog {
	return &JobLog{
		id:              id,
		jobLogFilePath:  jobLogFilePath,
		localInputPaths: localInputPaths,
		irodsTargetPath: irodsTargetPath,
		transferTasks:   map[string]*FileTransferTask{},
		createdTime:     time.Now(),
		writeMutex:      sync.Mutex{},
		isNewJob:        true,
	}
}

func NewJobLogFromLog(jobLogFilePath string) (*JobLog, error) {
	fh, err := os.OpenFile(jobLogFilePath, os.O_RDONLY, 0664)
	if err != nil {
		return nil, err
	}

	defer fh.Close()

	jobLog := &JobLog{
		id:              "",
		jobLogFilePath:  jobLogFilePath,
		localInputPaths: []string{},
		irodsTargetPath: "",
		transferTasks:   map[string]*FileTransferTask{},
		createdTime:     time.Now(),
		writeMutex:      sync.Mutex{},
		isNewJob:        true,
	}

	scanner := bufio.NewScanner(fh)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Bytes()

		if firstLine {
			firstLine = false

			jobLogHeader, err := NewJobLogHeaderFromJSON(line)
			if err != nil {
				return nil, err
			}

			jobLog.id = jobLogHeader.ID
			jobLog.localInputPaths = jobLogHeader.LocalInputPaths
			jobLog.irodsTargetPath = jobLogHeader.IRODSTargetPath
			jobLog.createdTime = jobLogHeader.CreatedTime
			jobLog.isNewJob = false
		}

		task, err := NewFileTransferTaskFromJSON(line)
		if err != nil {
			return nil, err
		}

		jobLog.transferTasks[task.LocalPath] = task
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return jobLog, nil
}

func NewJobLogHeaderFromJSON(jsonBytes []byte) (*JobLogHeader, error) {
	var jobLogHeader JobLogHeader
	err := json.Unmarshal(jsonBytes, &jobLogHeader)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return &jobLogHeader, nil
}

func (jobLogHeader *JobLogHeader) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(jobLogHeader)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return jsonBytes, nil
}

func GetDefaultJobLogPath(id string) string {
	dir := ""
	homedir, err := os.UserHomeDir()
	if err == nil {
		dir = filepath.Join(homedir, ".gocmd")
	}

	if len(dir) == 0 {
		dir = os.TempDir()
	}

	filename := fmt.Sprintf("gocmd_job_%s.log", id)
	return filepath.Join(dir, filename)
}

func (job *JobLog) MakeJobLogDir() error {
	dir := filepath.Dir(job.jobLogFilePath)
	return os.MkdirAll(dir, 0775)
}

func (job *JobLog) WriteHeader() error {
	job.writeMutex.Lock()
	defer job.writeMutex.Unlock()

	fh, err := os.OpenFile(job.jobLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}

	defer fh.Close()

	if job.isNewJob {
		jobLogHeader := JobLogHeader{
			ID:              job.id,
			CreatedTime:     job.createdTime,
			LocalInputPaths: job.localInputPaths,
			IRODSTargetPath: job.irodsTargetPath,
		}

		headerJsonBytes, err := jobLogHeader.ToJSON()
		if err != nil {
			return err
		}

		_, err = fh.Write(headerJsonBytes)
		if err != nil {
			return err
		}

		_, err = fh.Write([]byte("\n"))
		if err != nil {
			return err
		}

		job.isNewJob = false
	}
	return nil
}

func (job *JobLog) Write(task *FileTransferTask) error {
	job.writeMutex.Lock()
	defer job.writeMutex.Unlock()

	fh, err := os.OpenFile(job.jobLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}

	defer fh.Close()

	if job.isNewJob {
		jobLogHeader := JobLogHeader{
			ID:              job.id,
			CreatedTime:     job.createdTime,
			LocalInputPaths: job.localInputPaths,
			IRODSTargetPath: job.irodsTargetPath,
		}

		headerJsonBytes, err := jobLogHeader.ToJSON()
		if err != nil {
			return err
		}

		_, err = fh.Write(headerJsonBytes)
		if err != nil {
			return err
		}

		_, err = fh.Write([]byte("\n"))
		if err != nil {
			return err
		}

		job.isNewJob = false
	}

	jsonBytes, err := task.ToJSON()
	if err != nil {
		return err
	}

	_, err = fh.Write(jsonBytes)
	if err != nil {
		return err
	}

	_, err = fh.Write([]byte("\n"))
	if err != nil {
		return err
	}
	return nil
}

func (job *JobLog) IsCompleted(localPath string) bool {
	if task, ok := job.transferTasks[localPath]; ok {
		return task.Completed
	}
	return false
}

func (job *JobLog) PrintJobID() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "JobLog",
	})

	logger.Errorf("jobid: '%s'", job.id)
	fmt.Fprintln(os.Stderr, "TO CONTINUE FROM THIS POINT, USE FOLLOWING JOB ID")
	fmt.Fprintf(os.Stderr, "JOBID: %s\n", job.id)
}

func (job *JobLog) MonitorCtrlC() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	go func() {
		<-signalChannel

		job.PrintJobID()
		os.Exit(1)
	}()
}

func NewFileTransferTask(localPath string, irodsPath string, lastModified time.Time, size int64, hash string, completed bool) (*FileTransferTask, error) {
	return &FileTransferTask{
		LocalPath:        localPath,
		IRODSPath:        irodsPath,
		LastModifiedTime: lastModified,
		Size:             size,
		Hash:             hash,
		Completed:        completed,
	}, nil
}

func NewFileTransferTaskFromJSON(jsonBytes []byte) (*FileTransferTask, error) {
	var task FileTransferTask
	err := json.Unmarshal(jsonBytes, &task)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return &task, nil
}

func (task *FileTransferTask) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return jsonBytes, nil
}

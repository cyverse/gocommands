package commons

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileTransferTask struct {
	LocalPath        string    `json:"local_path"`
	IRODSPath        string    `json:"irods_path"`
	LastModifiedTime time.Time `json:"last_modified"`
	Size             int64     `json:"size"`
	Hash             string    `json:"hash"` // md5
	Completed        bool      `json:"completed"`
}

type Job struct {
	id             string
	jobLogFilePath string
	writeMutex     sync.Mutex
}

func NewJob(id string, jobLogFilePath string) *Job {
	return &Job{
		id:             id,
		jobLogFilePath: jobLogFilePath,
	}
}

func GetDefaultJobLogFilename(id string) string {
	tempDir := os.TempDir()
	filename := fmt.Sprintf("gocmd_job_%s.log", id)

	return filepath.Join(tempDir, filename)
}

func (job *Job) Write(task *FileTransferTask) error {
	job.writeMutex.Lock()
	defer job.writeMutex.Unlock()

	fh, err := os.OpenFile(job.jobLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}

	defer fh.Close()

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

func (job *Job) ReadAll() (map[string]*FileTransferTask, error) {
	fh, err := os.OpenFile(job.jobLogFilePath, os.O_RDONLY, 0664)
	if err != nil {
		return nil, err
	}

	defer fh.Close()

	taskMap := map[string]*FileTransferTask{}

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := scanner.Bytes()
		task, err := NewFileTransferTaskFromJSON(line)
		if err != nil {
			return nil, err
		}

		taskMap[task.LocalPath] = task
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return taskMap, nil
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

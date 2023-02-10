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

type BundleTransferLogHeader struct {
	ID              string    `json:"id"`
	CreatedTime     time.Time `json:"created_time"`
	LocalInputPaths []string  `json:"local_input_paths"`
	IRODSTargetPath string    `json:"irods_target_path"`
}

type BundleTransferLog struct {
	id              string
	logFilePath     string
	localInputPaths []string
	irodsTargetPath string
	transferTasks   map[string]*FileTransferTask
	createdTime     time.Time
	writeMutex      sync.Mutex
	isFirstWrite    bool
}

func NewBundleTransferLog(id string, logFilePath string, localInputPaths []string, irodsTargetPath string) *BundleTransferLog {
	return &BundleTransferLog{
		id:              id,
		logFilePath:     logFilePath,
		localInputPaths: localInputPaths,
		irodsTargetPath: irodsTargetPath,
		transferTasks:   map[string]*FileTransferTask{},
		createdTime:     time.Now(),
		writeMutex:      sync.Mutex{},
		isFirstWrite:    true,
	}
}

func NewBundleTransferLogFromLog(logFilePath string) (*BundleTransferLog, error) {
	fh, err := os.OpenFile(logFilePath, os.O_RDONLY, 0664)
	if err != nil {
		return nil, err
	}

	defer fh.Close()

	bundleTransferLog := &BundleTransferLog{
		id:              "",
		logFilePath:     logFilePath,
		localInputPaths: []string{},
		irodsTargetPath: "",
		transferTasks:   map[string]*FileTransferTask{},
		createdTime:     time.Now(),
		writeMutex:      sync.Mutex{},
		isFirstWrite:    true,
	}

	scanner := bufio.NewScanner(fh)
	firstLine := true
	for scanner.Scan() {
		line := scanner.Bytes()

		if firstLine {
			firstLine = false

			logHeader, err := NewBundleTransferLogHeaderFromJSON(line)
			if err != nil {
				return nil, err
			}

			bundleTransferLog.id = logHeader.ID
			bundleTransferLog.localInputPaths = logHeader.LocalInputPaths
			bundleTransferLog.irodsTargetPath = logHeader.IRODSTargetPath
			bundleTransferLog.createdTime = logHeader.CreatedTime
			bundleTransferLog.isFirstWrite = false
		}

		task, err := NewFileTransferTaskFromJSON(line)
		if err != nil {
			return nil, err
		}

		bundleTransferLog.transferTasks[task.LocalPath] = task
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return bundleTransferLog, nil
}

func NewBundleTransferLogHeaderFromJSON(jsonBytes []byte) (*BundleTransferLogHeader, error) {
	var logHeader BundleTransferLogHeader
	err := json.Unmarshal(jsonBytes, &logHeader)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return &logHeader, nil
}

func (logHeader *BundleTransferLogHeader) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(logHeader)
	if err != nil {
		return nil, fmt.Errorf("JSON Marshal Error - %v", err)
	}

	return jsonBytes, nil
}

func GetDefaultBundleTransferLogPath(id string) string {
	dir := ""
	homedir, err := os.UserHomeDir()
	if err == nil {
		dir = filepath.Join(homedir, ".gocmd")
	}

	if len(dir) == 0 {
		dir = os.TempDir()
	}

	filename := fmt.Sprintf("gocmd_bundle_transfer_%s.log", id)
	return filepath.Join(dir, filename)
}

func (bundleTransferLog *BundleTransferLog) MakeBundleTransferLogDir() error {
	dir := filepath.Dir(bundleTransferLog.logFilePath)
	return os.MkdirAll(dir, 0775)
}

func (bundleTransferLog *BundleTransferLog) WriteHeader() error {
	bundleTransferLog.writeMutex.Lock()
	defer bundleTransferLog.writeMutex.Unlock()

	fh, err := os.OpenFile(bundleTransferLog.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}

	defer fh.Close()

	if bundleTransferLog.isFirstWrite {
		logHeader := BundleTransferLogHeader{
			ID:              bundleTransferLog.id,
			CreatedTime:     bundleTransferLog.createdTime,
			LocalInputPaths: bundleTransferLog.localInputPaths,
			IRODSTargetPath: bundleTransferLog.irodsTargetPath,
		}

		headerJsonBytes, err := logHeader.ToJSON()
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

		bundleTransferLog.isFirstWrite = false
	}
	return nil
}

func (bundleTransferLog *BundleTransferLog) Write(task *FileTransferTask) error {
	bundleTransferLog.writeMutex.Lock()
	defer bundleTransferLog.writeMutex.Unlock()

	fh, err := os.OpenFile(bundleTransferLog.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}

	defer fh.Close()

	if bundleTransferLog.isFirstWrite {
		logHeader := BundleTransferLogHeader{
			ID:              bundleTransferLog.id,
			CreatedTime:     bundleTransferLog.createdTime,
			LocalInputPaths: bundleTransferLog.localInputPaths,
			IRODSTargetPath: bundleTransferLog.irodsTargetPath,
		}

		headerJsonBytes, err := logHeader.ToJSON()
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

		bundleTransferLog.isFirstWrite = false
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

func (bundleTransferLog *BundleTransferLog) IsCompleted(localPath string) bool {
	if task, ok := bundleTransferLog.transferTasks[localPath]; ok {
		return task.Completed
	}
	return false
}

func (bundleTransferLog *BundleTransferLog) PrintJobID() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "PrintJobID",
	})

	logger.Errorf("jobid: '%s'", bundleTransferLog.id)
	fmt.Fprintln(os.Stderr, "TO CONTINUE FROM THIS POINT, USE FOLLOWING JOB ID")
	fmt.Fprintf(os.Stderr, "JOBID: %s\n", bundleTransferLog.id)
}

func (bundleTransferLog *BundleTransferLog) MonitorCtrlC() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	go func() {
		<-signalChannel

		bundleTransferLog.PrintJobID()
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

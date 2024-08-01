package commons

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"golang.org/x/xerrors"
)

// TransferMethod determines transfer method
type TransferMethod string

const (
	// TransferMethodGet is for get
	TransferMethodGet TransferMethod = "GET"
	// TransferMethodPut is for put
	TransferMethodPut TransferMethod = "PUT"
	// TransferMethodBput is for bput command
	TransferMethodBput TransferMethod = "BPUT"
	// TransferMethodBputUnknown is for unknown command
	TransferMethodBputUnknown TransferMethod = "UNKNOWN"
)

type TransferReportFile struct {
	Method TransferMethod `json:"method"` // get, put, bput ...

	StartAt time.Time `json:"start_time"`
	EndAt   time.Time `json:"end_at"`

	LocalPath         string `json:"local_path"`
	IrodsPath         string `json:"irods_path"`
	ChecksumAlgorithm string `json:"checksum_algorithm"`
	LocalSize         int64  `json:"local_size"`
	LocalChecksum     string `json:"local_checksum"`
	IrodsSize         int64  `json:"irods_size"`
	IrodsChecksum     string `json:"irods_checksum"`

	Error error    `json:"error,omitempty"`
	Notes []string `json:"notes"` // additional notes
}

// GetTransferMethod returns transfer method
func GetTransferMethod(method string) TransferMethod {
	switch strings.ToUpper(method) {
	case string(TransferMethodGet), "DOWNLOAD", "DOWN":
		return TransferMethodGet
	case string(TransferMethodPut), "UPLOAD", "UP":
		return TransferMethodPut
	case string(TransferMethodBput), "BULK_UPLOAD":
		return TransferMethodBput
	default:
		return TransferMethodBputUnknown
	}
}

func NewTransferReportFileFromTransferResult(result *irodsclient_fs.FileTransferResult, method TransferMethod, err error, notes []string) *TransferReportFile {
	return &TransferReportFile{
		Method:            method,
		StartAt:           result.StartTime,
		EndAt:             result.EndTime,
		LocalPath:         result.LocalPath,
		LocalSize:         result.LocalSize,
		LocalChecksum:     hex.EncodeToString(result.LocalCheckSum),
		IrodsPath:         result.IRODSPath,
		IrodsSize:         result.IRODSSize,
		IrodsChecksum:     hex.EncodeToString(result.IRODSCheckSum),
		ChecksumAlgorithm: string(result.CheckSumAlgorithm),
		Error:             err,
		Notes:             notes,
	}
}

type TransferReportManager struct {
	reportPath     string
	report         bool
	reportToStdout bool

	writer io.WriteCloser
	lock   sync.Mutex
}

// NewTransferReportManager creates a new TransferReportManager
func NewTransferReportManager(report bool, reportPath string, reportToStdout bool) (*TransferReportManager, error) {
	var writer io.WriteCloser
	if !report {
		writer = nil
	} else if reportToStdout {
		// stdout
		writer = os.Stdout
	} else {
		// file
		fileWriter, err := os.Create(reportPath)
		if err != nil {
			return nil, xerrors.Errorf("failed to create a report file %s: %w", reportPath, err)
		}
		writer = fileWriter
	}

	manager := &TransferReportManager{
		report:         report,
		reportPath:     reportPath,
		reportToStdout: reportToStdout,

		writer: writer,
		lock:   sync.Mutex{},
	}

	return manager, nil
}

// Release releases resources
func (manager *TransferReportManager) Release() {
	if manager.writer != nil {
		if !manager.reportToStdout {
			manager.writer.Close()
		}

		manager.writer = nil
	}
}

// AddFile adds a new file transfer
func (manager *TransferReportManager) AddFile(file *TransferReportFile) error {
	if !manager.report {
		return nil
	}

	if manager.writer == nil {
		return nil
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	lineOutput := ""
	if manager.reportToStdout {
		// line print
		fmt.Printf("[%s]\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", file.Method, file.StartAt, file.EndAt, file.LocalPath, file.IrodsPath, file.LocalSize, file.IrodsSize, file.LocalChecksum, file.IrodsChecksum)
	} else {
		// json
		fileBytes, err := json.Marshal(file)
		if err != nil {
			return err
		}

		lineOutput = string(fileBytes) + "\n"
	}

	_, err := manager.writer.Write([]byte(lineOutput))
	if err != nil {
		return err
	}

	return nil
}

// AddTransfer adds a new file transfer
func (manager *TransferReportManager) AddTransfer(result *irodsclient_fs.FileTransferResult, method TransferMethod, err error, notes []string) error {
	file := NewTransferReportFileFromTransferResult(result, method, err, notes)
	return manager.AddFile(file)
}

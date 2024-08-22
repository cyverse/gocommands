package commons

import (
	"encoding/hex"
	"encoding/json"
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
	// TransferMethodCopy is for cp command
	TransferMethodCopy TransferMethod = "COPY"
	// TransferMethodDelete is for delete command
	TransferMethodDelete TransferMethod = "DELETE"
	// TransferMethodBputUnknown is for unknown command
	TransferMethodBputUnknown TransferMethod = "UNKNOWN"
)

type TransferReportFile struct {
	Method TransferMethod `json:"method"` // get, put, bput ...

	StartAt time.Time `json:"start_time"`
	EndAt   time.Time `json:"end_at"`

	SourcePath        string `json:"source_path"`
	DestPath          string `json:"dest_path"`
	ChecksumAlgorithm string `json:"checksum_algorithm"`
	SourceSize        int64  `json:"source_size"`
	SourceChecksum    string `json:"source_checksum"`
	DestSize          int64  `json:"dest_size"`
	DestChecksum      string `json:"dest_checksum"`

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
	case string(TransferMethodCopy), "CP":
		return TransferMethodCopy
	case string(TransferMethodDelete), "DEL":
		return TransferMethodDelete
	default:
		return TransferMethodBputUnknown
	}
}

func NewTransferReportFileFromTransferResult(result *irodsclient_fs.FileTransferResult, method TransferMethod, err error, notes []string) (*TransferReportFile, error) {
	if method == TransferMethodGet {
		// get
		// source is irods, target is local
		return &TransferReportFile{
			Method:  method,
			StartAt: result.StartTime,
			EndAt:   result.EndTime,

			SourcePath:     result.IRODSPath,
			SourceSize:     result.IRODSSize,
			SourceChecksum: hex.EncodeToString(result.IRODSCheckSum),

			DestPath:     result.LocalPath,
			DestSize:     result.LocalSize,
			DestChecksum: hex.EncodeToString(result.LocalCheckSum),

			ChecksumAlgorithm: string(result.CheckSumAlgorithm),
			Error:             err,
			Notes:             notes,
		}, nil
	} else if method == TransferMethodPut || method == TransferMethodBput {
		// put
		// source is local, target is irods
		return &TransferReportFile{
			Method:  method,
			StartAt: result.StartTime,
			EndAt:   result.EndTime,

			SourcePath:     result.LocalPath,
			SourceSize:     result.LocalSize,
			SourceChecksum: hex.EncodeToString(result.LocalCheckSum),

			DestPath:     result.IRODSPath,
			DestSize:     result.IRODSSize,
			DestChecksum: hex.EncodeToString(result.IRODSCheckSum),

			ChecksumAlgorithm: string(result.CheckSumAlgorithm),
			Error:             err,
			Notes:             notes,
		}, nil
	} else {
		return nil, xerrors.Errorf("unknown method %q", method)
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
			return nil, xerrors.Errorf("failed to create a report file %q: %w", reportPath, err)
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
		Printf("[%s]\t%s\t%s\t%s\t%d\t%s\t%s\t%d\t%s\n", file.Method, file.StartAt, file.EndAt, file.SourcePath, file.SourceSize, file.SourceChecksum, file.DestPath, file.DestSize, file.DestChecksum)
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
	file, err := NewTransferReportFileFromTransferResult(result, method, err, notes)
	if err != nil {
		return err
	}

	return manager.AddFile(file)
}

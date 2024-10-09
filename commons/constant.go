package commons

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	ClientProgramName          string                     = "gocommands"
	FilesystemTimeout          irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute)
	TransferThreadNumDefault   int                        = 5
	UploadThreadNumMax         int                        = 20
	TCPBufferSizeDefault       int                        = 4 * 1024 * 1024
	TCPBufferSizeStringDefault string                     = "4MB"
	RedirectToResourceMinSize  int64                      = 1024 * 1024 * 1024 // 1GB
)

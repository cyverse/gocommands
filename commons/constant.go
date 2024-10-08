package commons

import "time"

const (
	ClientProgramName          string        = "gocommands"
	FilesystemTimeout          time.Duration = 10 * time.Minute
	TransferThreadNumDefault   int           = 5
	UploadThreadNumMax         int           = 20
	TCPBufferSizeDefault       int           = 4 * 1024 * 1024
	TCPBufferSizeStringDefault string        = "4MB"
	RedirectToResourceMinSize  int64         = 1024 * 1024 * 1024 // 1GB
)

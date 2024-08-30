package commons

import "time"

const (
	clientProgramName          string        = "gocommands"
	connectionTimeout          time.Duration = 10 * time.Minute
	TransferTreadNumDefault    int           = 5
	UploadTreadNumMax          int           = 20
	TcpBufferSizeDefault       int           = 4 * 1024 * 1024
	TcpBufferSizeStringDefault string        = "4MB"

	RedirectToResourceMinSize int64 = 1024 * 1024 * 1024 // 1GB
)

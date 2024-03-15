package commons

const (
	TransferTreadNumDefault    int    = 5
	UploadTreadNumMax          int    = 20
	TcpBufferSizeDefault       int    = 4 * 1024 * 1024
	TcpBufferSizeStringDefault string = "4MB"

	RedirectToResourceMinSize int64 = 1024 * 1024 * 1024 // 1GB
	ParallelUploadMinSize     int64 = 80 * 1024 * 1024   // 80MB

)

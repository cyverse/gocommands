package commons

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

func UpdateIRODSFSClientTimeout(fs *irodsclient_fs.FileSystem, timeout int) {
	if timeout <= 0 {
		return
	}

	duration := time.Duration(timeout) * time.Second
	fs.GetConfig().MetadataConnection.OperationTimeout = irodsclient_types.Duration(duration)
	fs.GetConfig().IOConnection.OperationTimeout = irodsclient_types.Duration(duration)
}

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount, infiniteCache bool, longTimeout bool) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	if longTimeout {
		fsConfig.MetadataConnection.OperationTimeout = LongFilesystemTimeout
		fsConfig.IOConnection.OperationTimeout = LongFilesystemTimeout
	} else {
		fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
		fsConfig.IOConnection.OperationTimeout = FilesystemTimeout
	}

	// set tcp buffer size
	fsConfig.MetadataConnection.TCPBufferSize = GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TCPBufferSize = GetDefaultTCPBufferSize()

	if infiniteCache {
		// set infinite cache timeout
		infiniteDuration := irodsclient_types.Duration(365 * 24 * time.Hour) // 1y (almost infinite)

		fsConfig.Cache.Timeout = infiniteDuration
		fsConfig.Cache.CleanupTime = infiniteDuration
		fsConfig.Cache.InvalidateParentEntryCacheImmediately = true
		fsConfig.Cache.StartNewTransaction = false
	}

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientForLargeFileIO returns a file system client
func GetIRODSFSClientForLargeFileIO(account *irodsclient_types.IRODSAccount, maxIOConnection int, tcpBufferSize int) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = FilesystemTimeout

	// max connection for io
	if maxIOConnection <= 0 {
		maxIOConnection = irodsclient_fs.FileSystemIOConnectionMaxNumberDefault
	}
	fsConfig.IOConnection.MaxNumber = maxIOConnection

	// set tcp buffer size
	fsConfig.MetadataConnection.TCPBufferSize = tcpBufferSize
	fsConfig.IOConnection.TCPBufferSize = tcpBufferSize

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSConnection returns a connection
func GetIRODSConnection(account *irodsclient_types.IRODSAccount) (*irodsclient_conn.IRODSConnection, error) {
	conn := irodsclient_conn.NewIRODSConnection(account, time.Duration(FilesystemTimeout), ClientProgramName)
	err := conn.Connect()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}

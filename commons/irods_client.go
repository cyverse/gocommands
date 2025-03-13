package commons

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = FilesystemTimeout

	// set tcp buffer size
	fsConfig.MetadataConnection.TCPBufferSize = GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TCPBufferSize = GetDefaultTCPBufferSize()

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientForSingleOperation returns a file system client for single operation
func GetIRODSFSClientForSingleOperation(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = FilesystemTimeout

	// set tcp buffer size
	fsConfig.MetadataConnection.TCPBufferSize = GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TCPBufferSize = GetDefaultTCPBufferSize()

	// cache timeout
	// infinite
	infiniteDuration := irodsclient_types.Duration(365 * 24 * time.Hour) // 1y (almost infinite)

	fsConfig.Cache.Timeout = infiniteDuration
	fsConfig.Cache.CleanupTime = infiniteDuration
	fsConfig.Cache.InvalidateParentEntryCacheImmediately = true
	fsConfig.Cache.StartNewTransaction = false

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientForLargeFileIO returns a file system client
func GetIRODSFSClientForLargeFileIO(account *irodsclient_types.IRODSAccount, maxIOConnection int, tcpBufferSize int) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = FilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = FilesystemTimeout

	// max connection for io
	if maxIOConnection < irodsclient_fs.FileSystemIOConnectionMaxNumberDefault {
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

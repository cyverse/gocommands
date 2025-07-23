package irods

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/constant"
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
	fsConfig := irodsclient_fs.NewFileSystemConfig(constant.ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = config.FilesystemTimeout
	fsConfig.MetadataConnection.LongOperationTimeout = config.LongFilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = config.FilesystemTimeout
	fsConfig.IOConnection.LongOperationTimeout = config.LongFilesystemTimeout

	// set tcp buffer size
	fsConfig.MetadataConnection.TcpBufferSize = config.GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TcpBufferSize = config.GetDefaultTCPBufferSize()

	// set connection management
	fsConfig.MetadataConnection.WaitConnection = true
	fsConfig.IOConnection.WaitConnection = true

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
	fsConfig := irodsclient_fs.NewFileSystemConfig(constant.ClientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = config.FilesystemTimeout
	fsConfig.MetadataConnection.LongOperationTimeout = config.LongFilesystemTimeout
	fsConfig.IOConnection.OperationTimeout = config.FilesystemTimeout
	fsConfig.IOConnection.LongOperationTimeout = config.LongFilesystemTimeout

	// max connection for io
	if maxIOConnection < 1 {
		maxIOConnection = irodsclient_fs.FileSystemIOConnectionMaxNumberDefault
	}
	fsConfig.IOConnection.MaxNumber = maxIOConnection

	// set connection management
	fsConfig.MetadataConnection.WaitConnection = true
	fsConfig.IOConnection.WaitConnection = true

	// set tcp buffer size
	fsConfig.MetadataConnection.TcpBufferSize = tcpBufferSize
	fsConfig.IOConnection.TcpBufferSize = tcpBufferSize

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSConnection returns a connection
// used for init subcommand
func GetIRODSConnection(account *irodsclient_types.IRODSAccount) (*irodsclient_conn.IRODSConnection, error) {
	config := irodsclient_conn.IRODSConnectionConfig{
		ApplicationName: constant.ClientProgramName,
	}

	conn, err := irodsclient_conn.NewIRODSConnection(account, &config)
	if err != nil {
		return nil, xerrors.Errorf("failed to create connection: %w", err)
	}

	err = conn.Connect()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}

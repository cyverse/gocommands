package commons

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(clientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = filesystemTimeout
	fsConfig.IOConnection.OperationTimeout = filesystemTimeout

	// set tcp buffer size
	fsConfig.MetadataConnection.TCPBufferSize = TcpBufferSizeDefault
	fsConfig.IOConnection.TCPBufferSize = TcpBufferSizeDefault

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientAdvanced returns a file system client
func GetIRODSFSClientAdvanced(account *irodsclient_types.IRODSAccount, maxIOConnection int, tcpBufferSize int) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(clientProgramName)

	// set operation time out
	fsConfig.MetadataConnection.OperationTimeout = filesystemTimeout
	fsConfig.IOConnection.OperationTimeout = filesystemTimeout

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
	conn := irodsclient_conn.NewIRODSConnection(account, filesystemTimeout, clientProgramName)
	err := conn.Connect()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}

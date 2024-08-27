package commons

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(clientProgramName, irodsclient_fs.FileSystemConnectionErrorTimeoutDefault, irodsclient_fs.FileSystemConnectionInitNumberDefault, irodsclient_fs.FileSystemConnectionLifespanDefault,
		filesystemTimeout, filesystemTimeout, irodsclient_fs.FileSystemConnectionMaxDefault, TcpBufferSizeDefault,
		irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemTimeoutDefault, []irodsclient_fs.MetadataCacheTimeoutSetting{}, true, true)

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientAdvanced returns a file system client
func GetIRODSFSClientAdvanced(account *irodsclient_types.IRODSAccount, maxConnection int, tcpBufferSize int) (*irodsclient_fs.FileSystem, error) {
	if maxConnection < irodsclient_fs.FileSystemConnectionMaxDefault {
		maxConnection = irodsclient_fs.FileSystemConnectionMaxDefault
	}

	if tcpBufferSize < TcpBufferSizeDefault {
		tcpBufferSize = TcpBufferSizeDefault
	}

	fsConfig := irodsclient_fs.NewFileSystemConfig(clientProgramName, irodsclient_fs.FileSystemConnectionErrorTimeoutDefault, irodsclient_fs.FileSystemConnectionInitNumberDefault, irodsclient_fs.FileSystemConnectionLifespanDefault,
		filesystemTimeout, filesystemTimeout, maxConnection, tcpBufferSize,
		irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemTimeoutDefault, []irodsclient_fs.MetadataCacheTimeoutSetting{}, true, true)

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSConnection returns a connection
func GetIRODSConnection(account *irodsclient_types.IRODSAccount) (*irodsclient_conn.IRODSConnection, error) {
	conn := irodsclient_conn.NewIRODSConnection(account, connectionTimeout, clientProgramName)
	err := conn.Connect()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}

// TestConnect just test connection creation
func TestConnect(account *irodsclient_types.IRODSAccount) error {
	conn, err := GetIRODSConnection(account)
	if err != nil {
		return nil
	}

	defer conn.Disconnect()
	return nil
}

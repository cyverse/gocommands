package commons

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	ClientProgramName string        = "gocommands"
	connectionTimeout time.Duration = 10 * time.Minute
	filesystemTimeout time.Duration = 10 * time.Minute
)

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName, irodsclient_fs.ConnectionLifespanDefault,
		filesystemTimeout, filesystemTimeout, irodsclient_fs.FileSystemConnectionMaxDefault,
		irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemTimeoutDefault, []irodsclient_fs.MetadataCacheTimeoutSetting{}, false, true)

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSConnection returns a connection
func GetIRODSConnection(account *irodsclient_types.IRODSAccount) (*irodsclient_conn.IRODSConnection, error) {
	conn := irodsclient_conn.NewIRODSConnection(account, connectionTimeout, ClientProgramName)
	err := conn.Connect()
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// TestConnect just test connection creation
func TestConnect(account *irodsclient_types.IRODSAccount) error {
	conn, err := GetIRODSConnection(account)
	if err != nil {
		return err
	}

	defer conn.Disconnect()
	return nil
}

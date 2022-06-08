package commons

import (
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	ClientProgramName string = "gocommands"
)

// returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {

	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName, irodsclient_fs.ConnectionLifespanDefault,
		irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemConnectionMaxDefault,
		irodsclient_fs.FileSystemTimeoutDefault, irodsclient_fs.FileSystemTimeoutDefault, []irodsclient_fs.MetadataCacheTimeoutSetting{}, false, true)

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// TestConnect just test connection creation
func TestConnect(account *irodsclient_types.IRODSAccount) error {
	oneMin := 1 * time.Minute
	conn := irodsclient_conn.NewIRODSConnection(account, oneMin, ClientProgramName)

	err := conn.Connect()
	if err != nil {
		return err
	}

	defer conn.Disconnect()
	return nil
}

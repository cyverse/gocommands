package commons

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	goCommandsRepoPackagePath string = "cyverse/gocommands"

	ClientProgramName          string                     = "gocommands"
	FilesystemTimeout          irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute)
	transferThreadNumDefault   int                        = 5
	tcpBufferSizeStringDefault string                     = "1MB"
	bputForSyncDefaut          bool                       = false
	//RedirectToResourceMinSize  int64                      = 1024 * 1024 * 1024 // 1GB
)

func GetDefaultTCPBufferSize() int {
	size, _ := ParseSize(GetDefaultTCPBufferSizeString())
	return int(size)
}

func GetDefaultTCPBufferSizeString() string {
	// get from sysconfig
	sysConfig := GetSystemConfig()

	if sysConfig != nil && sysConfig.AdditionalConfig != nil {
		if sysConfig.AdditionalConfig.TCPBufferSize != "" {
			return sysConfig.AdditionalConfig.TCPBufferSize
		}
	}

	return tcpBufferSizeStringDefault
}

func GetDefaultTransferThreadNum() int {
	// get from sysconfig
	sysConfig := GetSystemConfig()

	if sysConfig != nil && sysConfig.AdditionalConfig != nil {
		if sysConfig.AdditionalConfig.TransferThreadNum > 0 {
			return sysConfig.AdditionalConfig.TransferThreadNum
		}
	}

	return transferThreadNumDefault
}

func GetDefaultBputForSync() bool {
	// get from sysconfig
	sysConfig := GetSystemConfig()

	if sysConfig != nil && sysConfig.AdditionalConfig != nil {
		if sysConfig.AdditionalConfig.BputForSync {
			return true
		}
	}

	return bputForSyncDefaut
}

func GetDefaultVerifyChecksum() bool {
	// get from sysconfig
	sysConfig := GetSystemConfig()

	if sysConfig != nil && sysConfig.AdditionalConfig != nil {
		if sysConfig.AdditionalConfig.VerifyChecksum {
			return true
		}
	}

	return false
}

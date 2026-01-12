package config

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons/types"
)

const (
	FilesystemTimeout               irodsclient_types.Duration = irodsclient_types.Duration(5 * time.Minute)
	LongFilesystemTimeout           irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute) // exceptionally long timeout for listing dirs or users
	transferThreadNumDefault        int                        = 5
	transferThreadNumPerFileDefault int                        = 5
	tcpBufferSizeStringDefault      string                     = "0"
	bputForSyncDefaut               bool                       = false

	MinFileNumInBundleDefault  int   = 3
	MaxFileNumInBundleDefault  int   = 50
	MaxFileSizeInBundleDefault int64 = 32 * 1024 * 1024       // 32MB
	MaxBundleFileSizeDefault   int64 = 2 * 1024 * 1024 * 1024 // 2GB
)

func GetDefaultFilesystemTimeout() int {
	return int(FilesystemTimeout / irodsclient_types.Duration(time.Second))
}

func GetDefaultTCPBufferSize() int {
	size, _ := types.ParseSize(GetDefaultTCPBufferSizeString())
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

func GetDefaultTransferThreadNumPerFile() int {
	// get from sysconfig
	sysConfig := GetSystemConfig()

	if sysConfig != nil && sysConfig.AdditionalConfig != nil {
		if sysConfig.AdditionalConfig.TransferThreadNumPerFile > 0 {
			return sysConfig.AdditionalConfig.TransferThreadNumPerFile
		}
	}

	return transferThreadNumPerFileDefault
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

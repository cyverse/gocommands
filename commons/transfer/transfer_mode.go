package transfer

import "strings"

type TransferMode string

const (
	TransferModeICAT   TransferMode = "icat"
	TransferModeWebDAV TransferMode = "webdav"
)

// GetTransferMode returns transfer mode
func GetTransferMode(mode string) TransferMode {
	switch strings.ToLower(mode) {
	case string(TransferModeICAT):
		return TransferModeICAT
	case string(TransferModeWebDAV), "http", "web":
		return TransferModeWebDAV
	default:
		return TransferModeICAT
	}
}

func (t TransferMode) Valid() bool {
	if t == TransferModeICAT || t == TransferModeWebDAV {
		return true
	}
	return false
}

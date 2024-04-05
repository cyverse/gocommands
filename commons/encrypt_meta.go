package commons

import (
	"strconv"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

type EncryptionConfig struct {
	Required bool
	Mode     EncryptionMode
}

// GetEncryptionConfigFromMeta returns encryption config from meta
func GetEncryptionConfigFromMeta(filesystem *irodsclient_fs.FileSystem, targetPath string) *EncryptionConfig {
	config := EncryptionConfig{
		Required: false,
		Mode:     EncryptionModeUnknown,
	}

	metas, err := filesystem.ListMetadata(targetPath)
	if err != nil {
		return &config
	}

	for _, meta := range metas {
		switch strings.ToLower(meta.Name) {
		case "encryption.required":
			bv, err := strconv.ParseBool(meta.Value)
			if err != nil {
				bv = false
			}

			config.Required = bv
		case "encryption.mode":
			config.Mode = GetEncryptionMode(meta.Value)
		}
	}

	return &config
}

package encryption

import (
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

type EncryptionConfig struct {
	Mode EncryptionMode
}

// GetEncryptionConfigFromMeta returns encryption config from meta
func GetEncryptionConfigFromMeta(filesystem *irodsclient_fs.FileSystem, targetPath string) *EncryptionConfig {
	config := EncryptionConfig{
		Mode: EncryptionModeNone,
	}

	metas, err := filesystem.ListMetadata(targetPath)
	if err != nil {
		return &config
	}

	for _, meta := range metas {
		switch strings.ToLower(meta.Name) {
		case "encryption.mode", "gocommands.encryption.mode", "encryption::mode", "gocommands::encryption::mode":
			config.Mode = GetEncryptionMode(meta.Value)
		}
	}

	return &config
}

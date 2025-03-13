package commons

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"golang.org/x/xerrors"
)

// GetSystemConfigPath returns the system-specific configuration file path for gocmd
func GetSystemConfigPath() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "/etc/gocmd/config.json", nil
	case "darwin": // macOS
		return "/Library/Application Support/gocmd/config.json", nil
	case "windows":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(configDir, "gocmd", "config.json"), nil
	default:
		return "/etc/gocmd/config.json", nil
	}
}

type AdditionalSystemConfig struct {
	TransferMode      TransferMode `json:"transfer_mode,omitempty" yaml:"transfer_mode,omitempty"`
	BputForSync       bool         `json:"bput_for_sync,omitempty" yaml:"bput_for_sync,omitempty"`
	TCPBufferSize     string       `json:"tcp_buffer_size,omitempty" yaml:"tcp_buffer_size,omitempty"`
	TransferThreadNum int          `json:"transfer_thread_num,omitempty" yaml:"transfer_thread_num,omitempty"`
	VerifyChecksum    bool         `json:"verify_checksum,omitempty" yaml:"verify_checksum,omitempty"`
}

// SystemConfig stores system-specific configuration
type SystemConfig struct {
	IRODSConfig      map[string]interface{}  `json:"irods_config,omitempty" yaml:"irods_config,omitempty"`
	AdditionalConfig *AdditionalSystemConfig `json:"additional_config,omitempty" yaml:"additional_config,omitempty"`
}

// NewSystemConfig reads SystemConfig from a JSON file located at default path
func NewSystemConfig() (*SystemConfig, error) {
	configPath, err := GetSystemConfigPath()
	if err != nil {
		return nil, xerrors.Errorf("failed to get system config path: %w", err)
	}

	st, err := os.Stat(configPath)
	if err != nil {
		// file does not exist or is not accessible
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, xerrors.Errorf("failed to stat %s: %w", configPath, err)
	}

	if st.IsDir() {
		// path is directory
		return nil, xerrors.Errorf("%s is a directory", configPath)
	}

	jsonBytes, err := os.ReadFile(configPath)
	if err != nil {
		// file is not accessible
		return nil, xerrors.Errorf("failed to read %s: %w", configPath, err)
	}

	systemConfig := SystemConfig{}

	err = json.Unmarshal(jsonBytes, &systemConfig)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal %s: %w", configPath, err)
	}

	return &systemConfig, nil
}

func (sysConfig *SystemConfig) GetIRODSConfig() *irodsclient_config.Config {
	irodsConfig := irodsclient_config.GetDefaultConfig()

	// convert struct to map
	jsonConfig, err := json.Marshal(sysConfig.IRODSConfig)
	if err != nil {
		return irodsConfig
	}

	mapConfig := map[string]interface{}{}
	err = json.Unmarshal(jsonConfig, &mapConfig)
	if err != nil {
		return irodsConfig
	}

	for k, v := range sysConfig.IRODSConfig {
		mapConfig[k] = v
	}

	// convert map to struct
	jsonConfig, err = json.Marshal(mapConfig)
	if err != nil {
		return irodsConfig
	}

	err = json.Unmarshal(jsonConfig, irodsConfig)
	if err != nil {
		return irodsConfig
	}

	return irodsConfig
}

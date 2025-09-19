package commons

import (
	"os"

	"golang.org/x/xerrors"

	"gopkg.in/yaml.v3"
)

const (
	IRODSEnvironmentFileEnvKey string = "IRODS_ENVIRONMENT_FILE"
)

// GetDefaultIRODSConfigPath returns default config path for the process environment
func GetDefaultIRODSConfigPath() string {
	// use the path specified in the environment if present
	if irodsEnvironmentFileEnvVal, ok := os.LookupEnv(IRODSEnvironmentFileEnvKey); ok {
		if len(irodsEnvironmentFileEnvVal) > 0 {
			return irodsEnvironmentFileEnvVal
		}
	}

	// fall back to the default path
	irodsConfigPath, err := ExpandHomeDir("~/.irods")
	if err != nil {
		return ""
	}
	return irodsConfigPath
}

// ConfigTypeIn stores data that user can input if missing
type ConfigTypeIn struct {
	Host     string `yaml:"irods_host,omitempty"`
	Port     int    `yaml:"irods_port,omitempty"`
	ZoneName string `yaml:"irods_zone_name,omitempty"`
	Username string `yaml:"irods_user_name,omitempty"`
	Password string `yaml:"irods_user_password,omitempty"`
}

// NewConfigTypeInFromYAML creates ConfigTypeIn from YAML
func NewConfigTypeInFromYAML(yamlBytes []byte) (*ConfigTypeIn, error) {
	config := &ConfigTypeIn{}

	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}

// ToYAML converts to YAML bytes
func (config *ConfigTypeIn) ToYAML() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal to YAML: %w", err)
	}
	return yamlBytes, nil
}

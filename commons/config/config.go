package config

import (
	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/commons/path"
	"gopkg.in/yaml.v3"
)

const (
	defaultIRODSConfigDirPath = "~/.irods"
)

// GetDefaultIRODSConfigPath returns default config path
func GetDefaultIRODSConfigPath() string {
	irodsConfigPath, err := path.ExpandLocalHomeDirPath(defaultIRODSConfigDirPath)
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
		return nil, errors.Wrapf(err, "failed to unmarshal YAML")
	}

	return config, nil
}

// ToYAML converts to YAML bytes
func (config *ConfigTypeIn) ToYAML() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal to YAML")
	}
	return yamlBytes, nil
}

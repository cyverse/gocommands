package commons

import (
	"encoding/json"
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

type Config struct {
	CurrentWorkingDir string `yaml:"irods_cwd,omitempty" json:"irods_cwd,omitempty" envconfig:"IRODS_CWD"`
	Host              string `yaml:"irods_host,omitempty" json:"irods_host,omitempty" envconfig:"IRODS_HOST"`
	Port              int    `yaml:"irods_port,omitempty" json:"irods_port,omitempty" envconfig:"IRODS_PORT"`
	Username          string `yaml:"irods_user_name,omitempty" json:"irods_user_name,omitempty" envconfig:"IRODS_USER_NAME"`
	Zone              string `yaml:"irods_zone_name,omitempty" json:"irods_zone_name,omitempty" envconfig:"IRODS_ZONE_NAME"`
	DefaultResource   string `yaml:"irods_default_resource,omitempty" json:"irods_default_resource,omitempty" envconfig:"IRODS_DEFAULT_RESOURCE"`
	LogLevel          int    `yaml:"irods_log_level,omitempty" json:"irods_log_level,omitempty" envconfig:"IRODS_LOG_LEVEL"`

	Password string `yaml:"irods_user_password,omitempty" json:"irods_user_password,omitempty" envconfig:"IRODS_USER_PASSWORD"`
}

type EnvConfig struct {
	ConfigPath           string `envconfig:"GOCMD_CONFIG_PATH"`
	IrodsEnvironmentFile string `envconfig:"IRODS_ENVIRONMENT_FILE"`
}

// NewConfigFromJSON creates Config from JSON
func NewConfigFromJSON(jsonBytes []byte) (*Config, error) {
	config := Config{
		Port: 1247,
	}

	err := json.Unmarshal(jsonBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON - %v", err)
	}

	return &config, nil
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(yamlBytes []byte) (*Config, error) {
	config := Config{
		Port: 1247,
	}

	err := yaml.Unmarshal(yamlBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML - %v", err)
	}

	return &config, nil
}

// NewConfigFromENV creates Config from Environmental variables
func NewConfigFromENV() (*Config, error) {
	config := Config{
		Port: 1247,
	}

	err := envconfig.Process("", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to read config from environmental variables - %v", err)
	}

	return &config, nil
}

// NewEnvConfigFromENV creates EnvConfig from Environmental variables
func NewEnvConfigFromENV() (*EnvConfig, error) {
	envConfig := EnvConfig{
		ConfigPath:           "",
		IrodsEnvironmentFile: "~/.irods/irods_environment.json",
	}

	err := envconfig.Process("", &envConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read config from environmental variables - %v", err)
	}

	return &envConfig, nil
}

package commons

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

// Config holds the parameters list which can be configured
type Config struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Zone     string `yaml:"zone"`
	Password string `yaml:"password"`
}

// NewDefaultConfig creates defaultConfig
func NewDefaultConfig() *Config {
	return &Config{
		Host: "data.cyverse.org",
		Port: 1247,
		Zone: "iplant",
	}
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(yamlBytes []byte) (*Config, error) {
	config := &Config{
		Host: "data.cyverse.org",
		Port: 1247,
		Zone: "iplant",
	}

	err := yaml.Unmarshal(yamlBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML - %v", err)
	}

	return config, nil
}

// Validate validates configuration
func (config *Config) Validate() error {
	if len(config.Host) == 0 {
		return fmt.Errorf("hostname must be given")
	}

	if config.Port <= 0 {
		return fmt.Errorf("port must be given")
	}

	if len(config.User) == 0 {
		return fmt.Errorf("user must be given")
	}

	if len(config.Zone) == 0 {
		return fmt.Errorf("zone must be given")
	}

	if len(config.Password) == 0 {
		return fmt.Errorf("password must be given")
	}

	return nil
}

package config

import (
	"testing"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
)

func TestApplyIRODSConfigOverridesPreservesUnspecifiedValues(t *testing.T) {
	systemConfig := &SystemConfig{
		IRODSConfig: map[string]interface{}{
			"irods_host": "system.example.org",
			"irods_port": 1247,
		},
	}
	baseConfig := irodsclient_config.GetDefaultConfig()
	baseConfig.Host = "user.example.org"
	baseConfig.Port = 9999
	baseConfig.Username = "user"

	updatedConfig, err := systemConfig.ApplyIRODSConfigOverrides(baseConfig)
	if err != nil {
		t.Fatalf("ApplyIRODSConfigOverrides() error = %v", err)
	}

	if updatedConfig.Host != "system.example.org" || updatedConfig.Port != 1247 {
		t.Errorf("system overrides = %q:%d, want system.example.org:1247", updatedConfig.Host, updatedConfig.Port)
	}
	if updatedConfig.Username != "user" {
		t.Errorf("Username = %q, want user", updatedConfig.Username)
	}
}

func TestApplyIRODSConfigOverridesReturnsInvalidTypeError(t *testing.T) {
	systemConfig := &SystemConfig{
		IRODSConfig: map[string]interface{}{"irods_port": "not-a-port"},
	}

	if _, err := systemConfig.ApplyIRODSConfigOverrides(irodsclient_config.GetDefaultConfig()); err == nil {
		t.Fatal("ApplyIRODSConfigOverrides() error = nil, want error for invalid port type")
	}
}

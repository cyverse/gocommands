package config

import (
	"testing"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
)

func TestSystemIRODSConfigProvidesDefaultsForEnvironmentFile(t *testing.T) {
	systemConfig := &SystemConfig{
		IRODSConfig: map[string]interface{}{
			"irods_host": "system.example.org",
			"irods_port": 1247,
		},
	}

	baseConfig, err := systemConfig.GetIRODSConfig()
	if err != nil {
		t.Fatalf("GetIRODSConfig() error = %v", err)
	}
	updatedConfig, err := irodsclient_config.NewConfigFromJSON(baseConfig, []byte(`{"irods_host":"user.example.org","irods_user_name":"user"}`))
	if err != nil {
		t.Fatalf("NewConfigFromJSON() error = %v", err)
	}

	if updatedConfig.Host != "user.example.org" || updatedConfig.Port != 1247 {
		t.Errorf("environment and system values = %q:%d, want user.example.org:1247", updatedConfig.Host, updatedConfig.Port)
	}
	if updatedConfig.Username != "user" {
		t.Errorf("Username = %q, want user", updatedConfig.Username)
	}
}

func TestApplyIRODSConfigOverridesReturnsInvalidTypeError(t *testing.T) {
	systemConfig := &SystemConfig{
		IRODSConfig: map[string]interface{}{"irods_port": "not-a-port"},
	}

	if _, err := systemConfig.GetIRODSConfig(); err == nil {
		t.Fatal("GetIRODSConfig() error = nil, want error for invalid port type")
	}
}

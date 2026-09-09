package config

import (
	"os"
	"testing"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
)

func TestInputMissingFieldsFromStdinPreservesConfiguredValues(t *testing.T) {
	if err := InitEnvironmentManager(); err != nil {
		t.Fatalf("InitEnvironmentManager() error = %v", err)
	}
	environmentManager.Environment.Host = "configured.example.org"
	environmentManager.Environment.Port = 1247
	environmentManager.Environment.ZoneName = "configuredZone"
	environmentManager.Environment.Username = "configuredUser"
	environmentManager.Environment.Password = "configuredPassword"

	stdin, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer stdin.Close()
	originalStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = originalStdin }()

	if err := InputMissingFieldsFromStdin(); err != nil {
		t.Fatalf("InputMissingFieldsFromStdin() error = %v", err)
	}
	if environmentManager.Environment.Host != "configured.example.org" ||
		environmentManager.Environment.Port != 1247 ||
		environmentManager.Environment.ZoneName != "configuredZone" ||
		environmentManager.Environment.Username != "configuredUser" ||
		environmentManager.Environment.Password != "configuredPassword" {
		t.Fatal("InputMissingFieldsFromStdin() replaced configured values with empty stdin values")
	}
}

func TestInputMissingFieldsFromStdinAppliesProvidedValues(t *testing.T) {
	if err := InitEnvironmentManager(); err != nil {
		t.Fatalf("InitEnvironmentManager() error = %v", err)
	}
	environmentManager.Environment.Host = "configured.example.org"
	environmentManager.Environment.Port = 1247
	environmentManager.Environment.ZoneName = "configuredZone"
	environmentManager.Environment.Username = "configuredUser"
	environmentManager.Environment.Password = "configuredPassword"

	stdin, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := stdin.WriteString("irods_host: stdin.example.org\nirods_port: 2000\nirods_user_name: stdinUser\n"); err != nil {
		stdin.Close()
		t.Fatalf("WriteString() error = %v", err)
	}
	if _, err := stdin.Seek(0, 0); err != nil {
		stdin.Close()
		t.Fatalf("Seek() error = %v", err)
	}
	defer stdin.Close()
	originalStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = originalStdin }()

	if err := InputMissingFieldsFromStdin(); err != nil {
		t.Fatalf("InputMissingFieldsFromStdin() error = %v", err)
	}
	if environmentManager.Environment.Host != "stdin.example.org" ||
		environmentManager.Environment.Port != 2000 ||
		environmentManager.Environment.Username != "stdinUser" {
		t.Fatal("InputMissingFieldsFromStdin() did not apply provided stdin values")
	}
	if environmentManager.Environment.ZoneName != "configuredZone" || environmentManager.Environment.Password != "configuredPassword" {
		t.Fatal("InputMissingFieldsFromStdin() replaced fields omitted from stdin")
	}
}

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

package subcmd

import (
	"os"
	"path/filepath"
	"testing"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
)

func TestRefreshCredentialsForInitRestoresConfiguredNativePassword(t *testing.T) {
	unsetEnvForTest(t, "IRODS_USER_PASSWORD")
	unsetEnvForTest(t, "IRODS_PAM_TOKEN")

	tempDir := t.TempDir()
	passwordFilePath := filepath.Join(tempDir, ".irodsA")
	if err := os.WriteFile(passwordFilePath, []byte("stored credentials"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	environmentFilePath := filepath.Join(tempDir, "irods_environment.json")
	if err := os.WriteFile(environmentFilePath, []byte(`{"irods_user_password":"configured-password","irods_pam_token":"configured-token"}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := &irodsclient_config.ICommandsEnvironmentManager{
		PasswordFilePath:    passwordFilePath,
		EnvironmentFilePath: environmentFilePath,
		Environment:         irodsclient_config.GetDefaultConfig(),
	}
	manager.Environment.Password = "stored-password"
	manager.Environment.PAMToken = "stored-token"

	if err := refreshCredentialsForInit(manager, true); err != nil {
		t.Fatalf("refreshCredentialsForInit() error = %v", err)
	}
	if manager.Environment.Password != "configured-password" {
		t.Errorf("password = %q, want configured password", manager.Environment.Password)
	}
	if manager.Environment.PAMToken != "" {
		t.Errorf("PAM token = %q, want empty", manager.Environment.PAMToken)
	}
}

func TestRefreshCredentialsForInitPrefersEnvironmentPassword(t *testing.T) {
	t.Setenv("IRODS_USER_PASSWORD", "environment-password")
	t.Setenv("IRODS_PAM_TOKEN", "")

	tempDir := t.TempDir()
	passwordFilePath := filepath.Join(tempDir, ".irodsA")
	if err := os.WriteFile(passwordFilePath, []byte("stored credentials"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	environmentFilePath := filepath.Join(tempDir, "irods_environment.json")
	if err := os.WriteFile(environmentFilePath, []byte(`{"irods_user_password":"configured-password"}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := &irodsclient_config.ICommandsEnvironmentManager{
		PasswordFilePath:    passwordFilePath,
		EnvironmentFilePath: environmentFilePath,
		Environment:         irodsclient_config.GetDefaultConfig(),
	}

	if err := refreshCredentialsForInit(manager, true); err != nil {
		t.Fatalf("refreshCredentialsForInit() error = %v", err)
	}
	if manager.Environment.Password != "environment-password" {
		t.Errorf("password = %q, want environment password", manager.Environment.Password)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) error = %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, value)
		}
	})
}

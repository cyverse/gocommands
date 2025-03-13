package commons

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

var (
	systemConfig       *SystemConfig
	environmentManager *irodsclient_config.ICommandsEnvironmentManager
)

func InitSystemConfig() error {
	sysConfig, err := NewSystemConfig()
	if err != nil {
		return err
	}

	systemConfig = sysConfig
	return nil
}

func GetSystemConfig() *SystemConfig {
	return systemConfig
}

// InitEnvironmentManager initializes envionment manager
func InitEnvironmentManager() error {
	manager, err := irodsclient_config.NewICommandsEnvironmentManager()
	if err != nil {
		return err
	}

	environmentManager = manager
	return nil
}

// InitEnvironmentManagerFromSystemConfig initializes envionment manager
func InitEnvironmentManagerFromSystemConfig() error {
	err := InitEnvironmentManager()
	if err != nil {
		return err
	}

	if systemConfig != nil {
		environmentManager.Environment = systemConfig.GetIRODSConfig()
	} else {
		environmentManager.Environment = irodsclient_config.GetDefaultConfig()
	}
	return nil
}

// GetEnvironmentManager returns environment manager
func GetEnvironmentManager() *irodsclient_config.ICommandsEnvironmentManager {
	return environmentManager
}

// GetSessionConfig returns session configuration
func GetSessionConfig() *irodsclient_config.Config {
	session, err := environmentManager.GetSessionConfig()
	if err != nil {
		return nil
	}

	return session
}

// GetCWD returns current working directory
func GetCWD() string {
	session, err := environmentManager.GetSessionConfig()
	if err != nil {
		return GetHomeDir()
	}

	if len(session.CurrentWorkingDir) > 0 {
		if !strings.HasPrefix(session.CurrentWorkingDir, "/") {
			// relative path from home
			currentWorkingDir := path.Join(GetHomeDir(), session.CurrentWorkingDir)
			return path.Clean(currentWorkingDir)
		}

		return path.Clean(session.CurrentWorkingDir)
	}

	return GetHomeDir()
}

// GetHomeDir returns home dir
func GetHomeDir() string {
	env := environmentManager.Environment
	if len(env.Home) > 0 {
		return env.Home
	}

	return fmt.Sprintf("/%s/home/%s", env.ClientZoneName, env.ClientUsername)
}

// SetCWD sets current workding directory
func SetCWD(cwd string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "SetCWD",
	})

	session := environmentManager.Session
	if !strings.HasPrefix(cwd, "/") {
		// relative path from home
		cwd = path.Join(GetHomeDir(), cwd)
	}

	session.CurrentWorkingDir = path.Clean(cwd)

	logger.Debugf("save session to file - id %d", environmentManager.PPID)
	err := environmentManager.SaveSession()
	if err != nil {
		return xerrors.Errorf("failed to save session: %w", err)
	}
	return nil
}

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	updated := false

	env := environmentManager.Environment
	if len(env.Host) == 0 {
		env.Host = Input("iRODS Host [data.cyverse.org]")
		if len(env.Host) == 0 {
			env.Host = "data.cyverse.org"
		}

		env.Port = InputInt("iRODS Port [1247]")
		if env.Port == 0 {
			env.Port = 1247
		}

		updated = true
	}

	if len(env.ZoneName) == 0 {
		env.ZoneName = Input("iRODS Zone [iplant]")
		if len(env.ZoneName) == 0 {
			env.ZoneName = "iplant"
		}

		updated = true
	}

	if len(env.Username) == 0 {
		env.Username = Input("iRODS Username")
		updated = true
	}

	password := environmentManager.Environment.Password
	pamToken := environmentManager.Environment.PAMToken
	if len(password) == 0 && len(pamToken) == 0 && env.Username != "anonymous" {
		environmentManager.Environment.Password = InputPassword("iRODS Password")
		updated = true
	}

	environmentManager.FixAuthConfiguration()

	return updated, nil
}

// InputMissingFieldsFromStdin inputs missing fields
func InputMissingFieldsFromStdin() error {
	// read from stdin
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values from stdin: %w", err)
	}

	configTypeIn, err := NewConfigTypeInFromYAML(stdinBytes)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values: %w", err)
	}

	environmentManager.Environment.Host = configTypeIn.Host
	environmentManager.Environment.Port = configTypeIn.Port
	environmentManager.Environment.ZoneName = configTypeIn.ZoneName
	environmentManager.Environment.Username = configTypeIn.Username
	environmentManager.Environment.Password = configTypeIn.Password

	environmentManager.FixAuthConfiguration()

	return nil
}

// ReinputFields re-inputs fields
func ReinputFields() (bool, error) {
	updated := false

	env := environmentManager.Environment
	if len(env.Host) == 0 {
		env.Host = "data.cyverse.org" // default
	}

	newHost := Input(fmt.Sprintf("iRODS Host [%s]", env.Host))
	if len(newHost) > 0 && newHost != env.Host {
		env.Host = newHost
		updated = true
	}

	if env.Port == 0 {
		env.Port = 1247 // default
	}

	newPort := InputInt(fmt.Sprintf("iRODS Port [%d]", env.Port))
	if newPort > 0 && newPort != env.Port {
		env.Port = newPort
		updated = true
	}

	if len(env.ZoneName) == 0 {
		env.ZoneName = "iplant" // default
	}

	newZone := Input(fmt.Sprintf("iRODS Zone [%s]", env.ZoneName))
	if len(newZone) > 0 && newZone != env.ZoneName {
		env.ZoneName = newZone
		updated = true
	}

	for {
		newUsername := ""
		if len(env.Username) > 0 {
			newUsername = Input(fmt.Sprintf("iRODS Username [%s]", env.Username))
		} else {
			newUsername = Input("iRODS Username")
		}

		if len(newUsername) > 0 && newUsername != env.Username {
			env.Username = newUsername
			updated = true
		}

		if len(env.Username) > 0 {
			break
		}
	}

	newPassword := InputPassword("iRODS Password")
	updated = true

	environmentManager.Environment.Password = newPassword

	environmentManager.FixAuthConfiguration()

	return updated, nil
}

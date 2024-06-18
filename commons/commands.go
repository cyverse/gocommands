package commons

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	irodsclient_icommands "github.com/cyverse/go-irodsclient/icommands"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/go-irodsclient/irods/util"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
	"golang.org/x/xerrors"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	environmentManager *irodsclient_icommands.ICommandsEnvironmentManager
	appConfig          *Config
	account            *irodsclient_types.IRODSAccount

	sessionID int
)

// GetEnvironmentManager returns environment manager
func GetEnvironmentManager() *irodsclient_icommands.ICommandsEnvironmentManager {
	return environmentManager
}

// GetConfig returns config
func GetConfig() *Config {
	return appConfig
}

// SetDefaultConfigIfEmpty sets default config if empty
func SetDefaultConfigIfEmpty() {
	if environmentManager == nil {
		iCommandsEnvMgr, _ := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		environmentManager = iCommandsEnvMgr
	}

	if appConfig == nil {
		appConfig = GetDefaultConfig()
		setConfigToICommandsEnvMgr(environmentManager, appConfig)
	}

	if environmentManager.Environment.LogLevel > 0 {
		logLevel := getLogrusLogLevel(environmentManager.Environment.LogLevel)
		log.SetLevel(logLevel)
	}

	environmentManager.Load(sessionID)
	setICommandsEnvMgrToConfig(appConfig, environmentManager)

	SyncAccount()
}

// SetSessionID sets session id
func SetSessionID(id int) {
	sessionID = id
}

// GetSessionID returns session id
func GetSessionID() int {
	return sessionID
}

// SyncAccount syncs irods account
func SyncAccount() error {
	newAccount, err := environmentManager.ToIRODSAccount()
	if err != nil {
		return xerrors.Errorf("failed to get account from iCommands Environment: %w", err)
	}

	if len(appConfig.ClientUsername) > 0 {
		newAccount.ClientUser = appConfig.ClientUsername
	}

	if len(appConfig.DefaultResource) > 0 {
		newAccount.DefaultResource = appConfig.DefaultResource
	}

	if len(appConfig.DefaultHashScheme) > 0 {
		newAccount.DefaultHashScheme = appConfig.DefaultHashScheme
	}

	if len(appConfig.Ticket) > 0 {
		newAccount.Ticket = appConfig.Ticket
	}

	account = newAccount
	return nil
}

// GetAccount returns irods account
func GetAccount() *irodsclient_types.IRODSAccount {
	return account
}

// GetCWD returns current working directory
func GetCWD() string {
	session := environmentManager.Session
	cwd := session.CurrentWorkingDir

	if len(cwd) == 0 {
		env := environmentManager.Environment
		cwd = env.CurrentWorkingDir
	}

	if len(cwd) > 0 {
		if !strings.HasPrefix(cwd, "/") {
			// relative path from home
			currentWorkingDir := path.Join(GetHomeDir(), cwd)
			return path.Clean(currentWorkingDir)
		}

		return path.Clean(cwd)
	}

	return GetHomeDir()
}

// GetZone returns zone
func GetZone() string {
	env := environmentManager.Environment
	return env.Zone
}

// GetUsername returns username
func GetUsername() string {
	env := environmentManager.Environment
	return env.Username
}

// GetHomeDir returns home dir
func GetHomeDir() string {
	env := environmentManager.Environment
	if len(env.Home) > 0 {
		return env.Home
	}

	return fmt.Sprintf("/%s/home/%s", env.Zone, env.Username)
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

	logger.Debugf("save session to file - id %d", sessionID)
	err := environmentManager.SaveSession(sessionID)
	if err != nil {
		return xerrors.Errorf("failed to save session: %w", err)
	}
	return nil
}

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	if environmentManager == nil {
		envMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		if err != nil {
			return false, xerrors.Errorf("failed to get new iCommands Environment: %w", err)
		}

		environmentManager = envMgr
		err = SyncAccount()
		if err != nil {
			return false, xerrors.Errorf("failed to get iCommands Environment: %w", err)
		}
	}

	updated := false

	env := environmentManager.Environment
	if len(env.Host) == 0 {
		fmt.Print("iRODS Host [data.cyverse.org]: ")
		fmt.Scanln(&env.Host)
		if len(env.Host) == 0 {
			env.Host = "data.cyverse.org"
		}

		fmt.Print("iRODS Port [1247]: ")
		fmt.Scanln(&env.Port)
		if env.Port == 0 {
			env.Port = 1247
		}

		updated = true
	}

	if len(env.Zone) == 0 {
		fmt.Print("iRODS Zone [iplant]: ")
		fmt.Scanln(&env.Zone)
		if len(env.Zone) == 0 {
			env.Zone = "iplant"
		}

		updated = true
	}

	if len(env.Username) == 0 {
		fmt.Print("iRODS Username: ")
		fmt.Scanln(&env.Username)
		updated = true
	}

	password := environmentManager.Password
	pamToken := environmentManager.PamToken
	if len(password) == 0 && len(pamToken) == 0 && env.Username != "anonymous" {
		fmt.Print("iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return false, xerrors.Errorf("failed to read password: %w", err)
		}

		fmt.Print("\n")
		password = string(bytePassword)
		updated = true
	}
	environmentManager.Password = password
	err := SyncAccount()
	if err != nil {
		return updated, xerrors.Errorf("failed to get iCommands Environment: %w", err)
	}

	return updated, nil
}

// InputMissingFieldsFromStdin inputs missing fields
func InputMissingFieldsFromStdin() error {
	if environmentManager == nil {
		envMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		if err != nil {
			return xerrors.Errorf("failed to get new iCommands Environment: %w", err)
		}

		environmentManager = envMgr
	}

	// read from stdin
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values from stdin: %w", err)
	}

	configTypeIn, err := NewConfigTypeInFromYAML(stdinBytes)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values: %w", err)
	}

	env := environmentManager.Environment
	env.Host = configTypeIn.Host
	env.Port = configTypeIn.Port
	env.Zone = configTypeIn.Zone
	env.Username = configTypeIn.Username
	environmentManager.Password = configTypeIn.Password

	return nil
}

// ReinputFields re-inputs fields
func ReinputFields() (bool, error) {
	if environmentManager == nil {
		envMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		if err != nil {
			return false, xerrors.Errorf("failed to get new iCommands Environment: %w", err)
		}

		environmentManager = envMgr
		err = SyncAccount()
		if err != nil {
			return false, xerrors.Errorf("failed to get iCommands Environment: %w", err)
		}
	}

	updated := false

	env := environmentManager.Environment
	if len(env.Host) == 0 {
		env.Host = "data.cyverse.org" // default
	}

	fmt.Printf("iRODS Host [%s]: ", env.Host)
	newHost := ""
	fmt.Scanln(&newHost)
	if len(newHost) > 0 && newHost != env.Host {
		env.Host = newHost
		updated = true
	}

	if env.Port == 0 {
		env.Port = 1247 // default
	}

	fmt.Printf("iRODS Port [%d]: ", env.Port)
	newPort := 0
	fmt.Scanln(&newPort)
	if newPort > 0 && newPort != env.Port {
		env.Port = newPort
		updated = true
	}

	if len(env.Zone) == 0 {
		env.Zone = "iplant" // default
	}

	fmt.Printf("iRODS Zone [%s]: ", env.Zone)
	newZone := ""
	fmt.Scanln(&newZone)
	if len(newZone) > 0 && newZone != env.Zone {
		env.Zone = newZone
		updated = true
	}

	for {
		if len(env.Username) > 0 {
			fmt.Printf("iRODS Username [%s]: ", env.Username)
		} else {
			fmt.Printf("iRODS Username: ")
		}

		newUsername := ""
		fmt.Scanln(&newUsername)
		if len(newUsername) > 0 && newUsername != env.Username {
			env.Username = newUsername
			updated = true
		}

		if len(env.Username) > 0 {
			break
		}
	}

	fmt.Print("iRODS Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return false, xerrors.Errorf("failed to read password: %w", err)
	}
	fmt.Print("\n")
	newPassword := string(bytePassword)
	updated = true

	environmentManager.Password = newPassword

	err = SyncAccount()
	if err != nil {
		return updated, xerrors.Errorf("failed to get iCommands Environment: %w", err)
	}

	return updated, nil
}

func isICommandsEnvDir(dirPath string) bool {
	realDirPath, err := ResolveSymlink(dirPath)
	if err != nil {
		return false
	}

	st, err := os.Stat(realDirPath)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if strings.HasPrefix(entry.Name(), "irods_environment.json.") {
				return true
			} else if entry.Name() == "irods_environment.json" {
				return true
			} else if entry.Name() == ".irodsA" {
				return true
			}
		}
	}

	return false
}

func isYAMLFile(filePath string) bool {
	st, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	if st.IsDir() {
		return false
	}

	ext := filepath.Ext(filePath)
	return ext == ".yaml" || ext == ".yml"
}

func setConfigToICommandsEnvMgr(envManager *irodsclient_icommands.ICommandsEnvironmentManager, config *Config) {
	envManager.Environment.CurrentWorkingDir = config.CurrentWorkingDir
	envManager.Environment.Host = config.Host
	envManager.Environment.Port = config.Port
	envManager.Environment.Username = config.Username
	envManager.Environment.Zone = config.Zone
	envManager.Environment.DefaultResource = config.DefaultResource
	envManager.Environment.DefaultHashScheme = config.DefaultHashScheme
	envManager.Environment.LogLevel = config.LogLevel
	envManager.Environment.AuthenticationScheme = config.AuthenticationScheme
	envManager.Environment.ClientServerNegotiation = config.ClientServerNegotiation
	envManager.Environment.ClientServerPolicy = config.ClientServerPolicy
	envManager.Environment.SSLCACertificateFile = config.SSLCACertificateFile
	envManager.Environment.SSLCACertificatePath = config.SSLCACertificatePath
	envManager.Environment.EncryptionKeySize = config.EncryptionKeySize
	envManager.Environment.EncryptionAlgorithm = config.EncryptionAlgorithm
	envManager.Environment.EncryptionSaltSize = config.EncryptionSaltSize
	envManager.Environment.EncryptionNumHashRounds = config.EncryptionNumHashRounds

	envManager.Password = config.Password
}

func overwriteConfigToICommandsEnvMgr(envManager *irodsclient_icommands.ICommandsEnvironmentManager, config *Config) {
	if len(config.CurrentWorkingDir) > 0 {
		envManager.Environment.CurrentWorkingDir = config.CurrentWorkingDir
	}

	if len(config.Host) > 0 {
		envManager.Environment.Host = config.Host
	}

	if config.Port > 0 {
		envManager.Environment.Port = config.Port
	}

	if len(config.Username) > 0 {
		envManager.Environment.Username = config.Username
	}

	if len(config.Zone) > 0 {
		envManager.Environment.Zone = config.Zone
	}

	if len(config.DefaultResource) > 0 {
		envManager.Environment.DefaultResource = config.DefaultResource
	}

	if len(config.DefaultHashScheme) > 0 {
		envManager.Environment.DefaultHashScheme = config.DefaultHashScheme
	}

	if config.LogLevel > 0 {
		envManager.Environment.LogLevel = config.LogLevel
	}

	if len(config.AuthenticationScheme) > 0 {
		envManager.Environment.AuthenticationScheme = config.AuthenticationScheme
	}

	if len(config.ClientServerNegotiation) > 0 {
		envManager.Environment.ClientServerNegotiation = config.ClientServerNegotiation
	}

	if len(config.ClientServerPolicy) > 0 {
		envManager.Environment.ClientServerPolicy = config.ClientServerPolicy
	}

	if len(config.SSLCACertificateFile) > 0 {
		envManager.Environment.SSLCACertificateFile = config.SSLCACertificateFile
	}

	if len(config.SSLCACertificatePath) > 0 {
		envManager.Environment.SSLCACertificatePath = config.SSLCACertificatePath
	}

	if config.EncryptionKeySize > 0 {
		envManager.Environment.EncryptionKeySize = config.EncryptionKeySize
	}

	if len(config.EncryptionAlgorithm) > 0 {
		envManager.Environment.EncryptionAlgorithm = config.EncryptionAlgorithm
	}

	if config.EncryptionSaltSize > 0 {
		envManager.Environment.EncryptionSaltSize = config.EncryptionSaltSize
	}

	if config.EncryptionNumHashRounds > 0 {
		envManager.Environment.EncryptionNumHashRounds = config.EncryptionNumHashRounds
	}

	if len(config.Password) > 0 {
		envManager.Password = config.Password
	}
}

func setICommandsEnvMgrToConfig(config *Config, envManager *irodsclient_icommands.ICommandsEnvironmentManager) {
	config.CurrentWorkingDir = envManager.Environment.CurrentWorkingDir
	config.Host = envManager.Environment.Host
	config.Port = envManager.Environment.Port
	config.Username = envManager.Environment.Username
	config.Zone = envManager.Environment.Zone
	config.DefaultResource = envManager.Environment.DefaultResource
	config.DefaultHashScheme = envManager.Environment.DefaultHashScheme
	config.LogLevel = envManager.Environment.LogLevel
	config.AuthenticationScheme = envManager.Environment.AuthenticationScheme
	config.ClientServerNegotiation = envManager.Environment.ClientServerNegotiation
	config.ClientServerPolicy = envManager.Environment.ClientServerPolicy
	config.SSLCACertificateFile = envManager.Environment.SSLCACertificateFile
	config.SSLCACertificatePath = envManager.Environment.SSLCACertificatePath
	config.EncryptionKeySize = envManager.Environment.EncryptionKeySize
	config.EncryptionAlgorithm = envManager.Environment.EncryptionAlgorithm
	config.EncryptionSaltSize = envManager.Environment.EncryptionSaltSize
	config.EncryptionNumHashRounds = envManager.Environment.EncryptionNumHashRounds

	config.Password = envManager.Password
}

func getLogrusLogLevel(irodsLogLevel int) log.Level {
	switch irodsLogLevel {
	case 0:
		return log.PanicLevel
	case 1:
		return log.FatalLevel
	case 2, 3:
		return log.ErrorLevel
	case 4, 5, 6:
		return log.WarnLevel
	case 7:
		return log.InfoLevel
	case 8:
		return log.DebugLevel
	case 9, 10:
		return log.TraceLevel
	}

	if irodsLogLevel < 0 {
		return log.PanicLevel
	}

	return log.TraceLevel
}

func LoadConfigFromFile(configPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "LoadConfigFromFile",
	})

	configPath, err := ExpandHomeDir(configPath)
	if err != nil {
		return xerrors.Errorf("failed to expand home dir for %s: %w", configPath, err)
	}

	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return xerrors.Errorf("failed to compute absolute path for %s: %w", configPath, err)
	}

	logger.Debugf("reading config file/dir - %s", configPath)
	// check if it is a file or a dir
	_, err = os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(configPath)
		}
		return xerrors.Errorf("failed to stat %s: %w", configPath, err)
	}

	if isYAMLFile(configPath) {
		logger.Debugf("reading gocommands YAML config file - %s", configPath)

		iCommandsEnvMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		if err != nil {
			return xerrors.Errorf("failed to create iCommands Environment: %w", err)
		}

		err = iCommandsEnvMgr.SetEnvironmentFilePath(configPath)
		if err != nil {
			return xerrors.Errorf("failed to set environment file path %s: %w", configPath, err)
		}

		// read session
		sessionFilePath := iCommandsEnvMgr.GetSessionFilePath(sessionID)
		if util.ExistFile(sessionFilePath) {
			session, err := irodsclient_icommands.CreateICommandsEnvironmentFromFile(sessionFilePath)
			if err != nil {
				return xerrors.Errorf("failed to create icommands environment from file %s: %w", sessionFilePath, err)
			}

			iCommandsEnvMgr.Session = session
		}

		// load from YAML
		yjBytes, err := os.ReadFile(configPath)
		if err != nil {
			return xerrors.Errorf("failed to read file %s: %w", configPath, err)
		}

		defaultConfig := GetDefaultConfig()
		config, err := NewConfigFromYAML(defaultConfig, yjBytes)
		if err != nil {
			return xerrors.Errorf("failed to read config from YAML: %w", err)
		}

		setConfigToICommandsEnvMgr(iCommandsEnvMgr, config)

		if iCommandsEnvMgr.Environment.LogLevel > 0 {
			logLevel := getLogrusLogLevel(iCommandsEnvMgr.Environment.LogLevel)
			log.SetLevel(logLevel)
		}

		environmentManager = iCommandsEnvMgr
		appConfig = config

		err = SyncAccount()
		if err != nil {
			return xerrors.Errorf("failed to sync account: %w", err)
		}

		return nil
	}

	// icommands compatible
	configFilePath := configPath
	if isICommandsEnvDir(configPath) {
		configFilePath = filepath.Join(configPath, "irods_environment.json")
	}

	logger.Debugf("reading icommands environment file - %s", configFilePath)

	iCommandsEnvMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
	if err != nil {
		return xerrors.Errorf("failed to create new iCommands Environment: %w", err)
	}

	err = iCommandsEnvMgr.SetEnvironmentFilePath(configFilePath)
	if err != nil {
		return xerrors.Errorf("failed to set iCommands Environment file %s: %w", configFilePath, err)
	}

	err = iCommandsEnvMgr.Load(sessionID)
	if err != nil {
		return xerrors.Errorf("failed to read iCommands Environment: %w", err)
	}

	config := GetDefaultConfig()

	setICommandsEnvMgrToConfig(config, iCommandsEnvMgr)

	if iCommandsEnvMgr.Environment.LogLevel > 0 {
		logLevel := getLogrusLogLevel(iCommandsEnvMgr.Environment.LogLevel)
		log.SetLevel(logLevel)
	}

	environmentManager = iCommandsEnvMgr
	appConfig = config

	err = SyncAccount()
	if err != nil {
		return xerrors.Errorf("failed to sync account: %w", err)
	}

	return nil
}

// LoadAndOverwriteConfigFromEnv loads config from env and overwrites to existing env
func LoadAndOverwriteConfigFromEnv() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "LoadAndOverwriteConfigFromEnv",
	})

	logger.Debug("reading config from environment variables")

	config, err := NewConfigFromENV()
	if err != nil {
		return xerrors.Errorf("failed to get new iCommands Environment: %w", err)
	}

	overwriteConfigToICommandsEnvMgr(environmentManager, config)

	if environmentManager.Environment.LogLevel > 0 {
		logLevel := getLogrusLogLevel(environmentManager.Environment.LogLevel)
		log.SetLevel(logLevel)
	}

	setICommandsEnvMgrToConfig(appConfig, environmentManager)

	SyncAccount()

	return nil
}

func PrintAccount() error {
	envMgr := GetEnvironmentManager()
	if envMgr == nil {
		return xerrors.Errorf("environment is not set")
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendRows([]table.Row{
		{
			"iRODS Host",
			envMgr.Environment.Host,
		},
		{
			"iRODS Port",
			envMgr.Environment.Port,
		},
		{
			"iRODS Zone",
			envMgr.Environment.Zone,
		},
		{
			"iRODS Username",
			envMgr.Environment.Username,
		},
		{
			"iRODS Authentication Scheme",
			envMgr.Environment.AuthenticationScheme,
		},
	}, table.RowConfig{})
	t.Render()
	return nil
}

func PrintEnvironment() error {
	envMgr := GetEnvironmentManager()
	if envMgr == nil {
		return xerrors.Errorf("environment is not set")
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendRows([]table.Row{
		{
			"iRODS Session Environment File",
			envMgr.GetSessionFilePath(os.Getppid()),
		},
		{
			"iRODS Environment File",
			envMgr.GetEnvironmentFilePath(),
		},
		{
			"iRODS Host",
			envMgr.Environment.Host,
		},
		{
			"iRODS Port",
			envMgr.Environment.Port,
		},
		{
			"iRODS Zone",
			envMgr.Environment.Zone,
		},
		{
			"iRODS Username",
			envMgr.Environment.Username,
		},
		{
			"iRODS Default Resource",
			envMgr.Environment.DefaultResource,
		},
		{
			"iRODS Default Hash Scheme",
			envMgr.Environment.DefaultHashScheme,
		},
		{
			"iRODS Authentication Scheme",
			envMgr.Environment.AuthenticationScheme,
		},
		{
			"iRODS Client Server Negotiation",
			envMgr.Environment.ClientServerNegotiation,
		},
		{
			"iRODS Client Server Policy",
			envMgr.Environment.ClientServerPolicy,
		},
		{
			"iRODS SSL CA Certification File",
			envMgr.Environment.SSLCACertificateFile,
		},
		{
			"iRODS SSL CA Certification Path",
			envMgr.Environment.SSLCACertificatePath,
		},
		{
			"iRODS SSL Encryption Key Size",
			envMgr.Environment.EncryptionKeySize,
		},
		{
			"iRODS SSL Encryption Key Algorithm",
			envMgr.Environment.EncryptionAlgorithm,
		},
		{
			"iRODS SSL Encryption Salt Size",
			envMgr.Environment.EncryptionSaltSize,
		},
		{
			"iRODS SSL Encryption Hash Rounds",
			envMgr.Environment.EncryptionNumHashRounds,
		},
	}, table.RowConfig{})
	t.Render()
	return nil
}

// InputYN inputs Y or N
// true for Y, false for N
func InputYN(msg string) bool {
	userInput := ""

	for {
		fmt.Printf("%s [y/n]: ", msg)

		fmt.Scanln(&userInput)
		userInput = strings.ToLower(userInput)

		if userInput == "y" || userInput == "yes" {
			return true
		} else if userInput == "n" || userInput == "no" {
			return false
		}
	}
}

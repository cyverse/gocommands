package commons

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/go-irodsclient/irods/util"
	irodsclient_icommands "github.com/cyverse/go-irodsclient/utils/icommands"
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

func GetEnvironmentManager() *irodsclient_icommands.ICommandsEnvironmentManager {
	return environmentManager
}

func GetConfig() *Config {
	return appConfig
}

func SetDefaultConfigIfEmpty() {
	if environmentManager == nil {
		iCommandsEnvMgr, _ := irodsclient_icommands.CreateIcommandsEnvironmentManager()
		iCommandsEnvMgr.SetEnvironmentFilePath("./")
		environmentManager = iCommandsEnvMgr
	}

	if appConfig == nil {
		appConfig = GetDefaultConfig()
		setConfigToICommandsEnvMgr(environmentManager, appConfig)
	}
}

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

	if len(appConfig.Ticket) > 0 {
		newAccount.Ticket = appConfig.Ticket
	}

	account = newAccount
	return nil
}

func GetAccount() *irodsclient_types.IRODSAccount {
	return account
}

func getCWD(env *irodsclient_icommands.ICommandsEnvironment) string {
	currentWorkingDir := env.CurrentWorkingDir
	if len(currentWorkingDir) == 0 {
		return ""
	}

	if !strings.HasPrefix(currentWorkingDir, "/") {
		// relative path from home
		currentWorkingDir = fmt.Sprintf("/%s/home/%s/%s", env.Zone, env.Username, currentWorkingDir)
	}

	return path.Clean(currentWorkingDir)
}

func GetCWD() string {
	session := environmentManager.Session
	sessionPath := getCWD(session)

	if len(sessionPath) > 0 {
		return sessionPath
	}

	env := environmentManager.Environment
	envPath := getCWD(env)

	if len(envPath) == 0 {
		// set new
		return fmt.Sprintf("/%s/home/%s", env.Zone, env.Username)
	}
	return envPath
}

func GetZone() string {
	env := environmentManager.Environment
	return env.Zone
}

func GetUsername() string {
	env := environmentManager.Environment
	return env.Username
}

func GetHomeDir() string {
	env := environmentManager.Environment
	return fmt.Sprintf("/%s/home/%s", env.Zone, env.Username)
}

func SetCWD(cwd string) {
	env := environmentManager.Environment
	session := environmentManager.Session
	if !strings.HasPrefix(cwd, "/") {
		// relative path from home
		cwd = fmt.Sprintf("/%s/home/%s/%s", env.Zone, env.Username, cwd)
	}

	session.CurrentWorkingDir = path.Clean(cwd)
	environmentManager.SaveSession(sessionID)
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
	if len(password) == 0 && env.Username != "anonymous" {
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

func isICommandsEnvDir(filePath string) bool {
	st, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return false
	}

	envFilePath := filepath.Join(filePath, "irods_environment.json")
	passFilePath := filepath.Join(filePath, ".irodsA")

	stEnv, err := os.Stat(envFilePath)
	if err != nil {
		return false
	}

	if stEnv.IsDir() {
		return false
	}

	stPass, err := os.Stat(passFilePath)
	if err != nil {
		return false
	}

	if stPass.IsDir() {
		return false
	}

	return true
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
	envManager.Environment.LogLevel = config.LogLevel
	envManager.Environment.AuthenticationScheme = config.AuthenticationScheme
	envManager.Environment.ClientServerNegotiation = config.ClientServerNegotiation
	envManager.Environment.ClientServerPolicy = config.ClientServerPolicy
	envManager.Environment.SSLCACertificateFile = config.SSLCACertificateFile
	envManager.Environment.EncryptionKeySize = config.EncryptionKeySize
	envManager.Environment.EncryptionAlgorithm = config.EncryptionAlgorithm
	envManager.Environment.EncryptionSaltSize = config.EncryptionSaltSize
	envManager.Environment.EncryptionNumHashRounds = config.EncryptionNumHashRounds

	envManager.Password = config.Password
}

func setICommandsEnvMgrToConfig(config *Config, envManager *irodsclient_icommands.ICommandsEnvironmentManager) {
	config.CurrentWorkingDir = envManager.Environment.CurrentWorkingDir
	config.Host = envManager.Environment.Host
	config.Port = envManager.Environment.Port
	config.Username = envManager.Environment.Username
	config.Zone = envManager.Environment.Zone
	config.DefaultResource = envManager.Environment.DefaultResource
	config.LogLevel = envManager.Environment.LogLevel
	config.AuthenticationScheme = envManager.Environment.AuthenticationScheme
	config.ClientServerNegotiation = envManager.Environment.ClientServerNegotiation
	config.ClientServerPolicy = envManager.Environment.ClientServerPolicy
	config.SSLCACertificateFile = envManager.Environment.SSLCACertificateFile
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

		config, err := NewConfigFromYAML(yjBytes)
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

func LoadConfigFromEnv() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "LoadConfigFromEnv",
	})

	logger.Debug("reading config from environment variables")

	iCommandsEnvMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager()
	if err != nil {
		return xerrors.Errorf("failed to get new iCommands Environment: %w", err)
	}

	config, err := NewConfigFromENV()
	if err != nil {
		return xerrors.Errorf("failed to get new iCommands Environment: %w", err)
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
		return xerrors.Errorf("failed to get iCommands Environment: %w", err)
	}

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

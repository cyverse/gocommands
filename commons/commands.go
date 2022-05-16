package commons

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_icommands "github.com/cyverse/go-irodsclient/utils/icommands"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	environmentMgr *irodsclient_icommands.ICommandsEnvironmentManager
	account        *irodsclient_types.IRODSAccount

	resourceServer string
)

func GetEnvironmentManager() *irodsclient_icommands.ICommandsEnvironmentManager {
	return environmentMgr
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

	return filepath.Clean(currentWorkingDir)
}

func GetCWD() string {
	session := environmentMgr.Session
	sessionPath := getCWD(session)

	if len(sessionPath) > 0 {
		return sessionPath
	}

	env := environmentMgr.Environment
	envPath := getCWD(env)

	if len(envPath) == 0 {
		// set new
		return fmt.Sprintf("/%s/home/%s", env.Zone, env.Username)
	}
	return envPath
}

func SetCWD(cwd string) {
	env := environmentMgr.Environment
	session := environmentMgr.Session
	if !strings.HasPrefix(cwd, "/") {
		// relative path from home
		cwd = fmt.Sprintf("/%s/home/%s/%s", env.Zone, env.Username, cwd)
	}

	session.CurrentWorkingDir = filepath.Clean(cwd)
	environmentMgr.SaveSession()
}

func SetCommonFlags(command *cobra.Command) {
	command.Flags().StringP("config", "c", "", "Set config file (default is $HOME/.irods/irods_environment.json)")
	command.Flags().BoolP("version", "v", false, "Print version")
	command.Flags().BoolP("help", "h", false, "Print help")
	command.Flags().BoolP("debug", "d", false, "Enable debug mode")
	command.Flags().StringP("resource", "", "", "Set resource server")
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "ProcessCommonFlags",
	})

	debugFlag := command.Flags().Lookup("debug")
	if debugFlag != nil {
		debug, err := strconv.ParseBool(debugFlag.Value.String())
		if err != nil {
			debug = false
		}

		if debug {
			log.SetLevel(log.DebugLevel)
		}
	}

	helpFlag := command.Flags().Lookup("help")
	if helpFlag != nil {
		help, err := strconv.ParseBool(helpFlag.Value.String())
		if err != nil {
			help = false
		}

		if help {
			printHelp(command)
			return false, nil // stop here
		}
	}

	versionFlag := command.Flags().Lookup("version")
	if versionFlag != nil {
		version, err := strconv.ParseBool(versionFlag.Value.String())
		if err != nil {
			version = false
		}

		if version {
			printVersion(command)
			return false, nil // stop here
		}
	}

	configFlag := command.Flags().Lookup("config")
	if configFlag != nil {
		config := configFlag.Value.String()
		if len(config) > 0 {
			err := loadConfigFile(command, config)
			if err != nil {
				logger.Error(err)
				return false, err // stop here
			}
		}
	}

	if configFlag == nil || len(configFlag.Value.String()) == 0 {
		// auto detect
		homePath, err := os.UserHomeDir()
		if err == nil {
			irodsPath := filepath.Join(homePath, ".irods")
			loadConfigFile(command, irodsPath)
			//if err != nil {
			//logger.Error(err)
			// ignore error
			//}
		}
	}

	resourceFlag := command.Flags().Lookup("resource")
	if resourceFlag != nil {
		// load to global variable
		resourceServer = resourceFlag.Value.String()
	}

	return true, nil // contiue
}

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	if environmentMgr == nil {
		envMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManagerWithDefault()
		if err != nil {
			return false, err
		}

		environmentMgr = envMgr
		account, err = envMgr.ToIRODSAccount()
		if err != nil {
			return false, err
		}
	}

	updated := false

	env := environmentMgr.Environment
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

	for len(env.Username) == 0 {
		fmt.Print("iRODS Username: ")
		fmt.Scanln(&env.Username)
		if len(env.Username) == 0 {
			fmt.Println("Please provide username")
			fmt.Println("")
		} else {
			updated = true
		}
	}

	password := environmentMgr.Password
	for len(password) == 0 {
		fmt.Print("iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return false, err
		}

		fmt.Print("\n")
		password = string(bytePassword)

		if len(password) == 0 {
			fmt.Println("Please provide password")
			fmt.Println("")
		} else {
			updated = true
		}
	}
	environmentMgr.Password = password

	newAccount, err := environmentMgr.ToIRODSAccount()
	if err != nil {
		return updated, err
	}

	if len(resourceServer) > 0 {
		newAccount.DefaultResource = resourceServer
	}

	account = newAccount
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

func loadConfigFile(command *cobra.Command, configFilePath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "loadConfigFile",
	})

	logger.Debugf("reading config file - %s", configFilePath)
	// check if it is a file or a dir
	_, err := os.Stat(configFilePath)
	if err != nil {
		return err
	}

	if isICommandsEnvDir(configFilePath) {
		logger.Debugf("reading icommands environment file - %s", configFilePath)
		iCommandsEnvMgr, err := irodsclient_icommands.CreateIcommandsEnvironmentManager(configFilePath, 0)
		if err != nil {
			return err
		}

		err = iCommandsEnvMgr.Load()
		if err != nil {
			return err
		}

		if iCommandsEnvMgr.Environment.LogLevel > 0 {
			logLevel := log.Level(iCommandsEnvMgr.Environment.LogLevel / 2)
			log.SetLevel(logLevel)
		}

		loadedAccount, err := iCommandsEnvMgr.ToIRODSAccount()
		if err != nil {
			return err
		}

		environmentMgr = iCommandsEnvMgr
		account = loadedAccount
		return nil
	}

	return fmt.Errorf("unhandled configuration file - %s", configFilePath)
}

func printVersion(command *cobra.Command) error {
	info, err := GetVersionJSON()
	if err != nil {
		return err
	}

	fmt.Println(info)
	return nil
}

func printHelp(command *cobra.Command) error {
	return command.Usage()
}

func PrintAccount(command *cobra.Command) error {
	envMgr := GetEnvironmentManager()
	if envMgr == nil {
		return errors.New("environment is not set")
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
	}, table.RowConfig{})
	t.Render()
	return nil
}

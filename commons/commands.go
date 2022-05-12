package commons

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_icommands "github.com/cyverse/go-irodsclient/utils/icommands"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	account *irodsclient_types.IRODSAccount
)

func GetAccount() *irodsclient_types.IRODSAccount {
	return account
}

func SetCommonFlags(command *cobra.Command) {
	command.Flags().StringP("config", "c", "", "config file (default is $HOME/.irods/irods_environment.json)")
	command.Flags().BoolP("version", "v", false, "Print version")
	command.Flags().BoolP("help", "h", false, "Print help")
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "ProcessCommonFlags",
	})

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
			err := loadConfigFile(command, irodsPath)
			if err != nil {
				logger.Error(err)
				// ignore error
			}
		}
	}

	return true, nil // contiue
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
	st, err := os.Stat(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// not exists
			return err
		}
	}

	if isICommandsEnvDir(configFilePath) {
		logger.Debugf("reading icommands environment file - %s", configFilePath)
		loadedAccount, err := irodsclient_icommands.CreateAccountFromDir(configFilePath, 0)
		if err != nil {
			return err
		}

		account = loadedAccount
		return nil
	}

	if !st.IsDir() {
		logger.Debugf("reading gocommands config file - %s", configFilePath)
		// file
		// Read account configuration from YAML file
		yamlBytes, err := ioutil.ReadFile(configFilePath)
		if err != nil {
			return err
		}

		loadedAccount, err := irodsclient_types.CreateIRODSAccountFromYAML(yamlBytes)
		if err != nil {
			return err
		}

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

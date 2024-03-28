package flag

import (
	"fmt"
	"os"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

type CommonFlagValues struct {
	ConfigFilePath  string
	ShowVersion     bool
	ShowHelp        bool
	DebugMode       bool
	logLevelInput   string
	LogLevel        log.Level
	LogLevelUpdated bool
	SessionID       int
	Resource        string
	ResourceUpdated bool
}

const (
	IRODSEnvironmentFileEnvKey string = "IRODS_ENVIRONMENT_FILE"
)

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command, noResource bool) {
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", "", "Set config file or dir (default \"$HOME/.irods\")")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Set session ID")

	if !noResource {
		command.Flags().StringVarP(&commonFlagValues.Resource, "resource", "R", "", "Set resource server")
	}

	command.MarkFlagsMutuallyExclusive("debug", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")

	if !noResource {
		command.MarkFlagsMutuallyExclusive("resource", "version")
	}

	command.MarkFlagsMutuallyExclusive("session", "version")
}

func GetCommonFlagValues(command *cobra.Command) *CommonFlagValues {
	if len(commonFlagValues.logLevelInput) > 0 {
		lvl, err := log.ParseLevel(commonFlagValues.logLevelInput)
		if err != nil {
			lvl = log.InfoLevel
		}
		commonFlagValues.LogLevel = lvl
		commonFlagValues.LogLevelUpdated = true
	}

	if command.Flags().Changed("resource") {
		commonFlagValues.ResourceUpdated = true
	}

	return &commonFlagValues
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "flag",
		"function": "ProcessCommonFlags",
	})

	myCommonFlagValues := GetCommonFlagValues(command)
	retryFlagValues := GetRetryFlagValues()

	if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}

	if myCommonFlagValues.ShowHelp {
		command.Usage()
		return false, nil // stop here
	}

	if myCommonFlagValues.ShowVersion {
		printVersion()
		return false, nil // stop here
	}

	logger.Debugf("use sessionID - %d", myCommonFlagValues.SessionID)
	commons.SetSessionID(myCommonFlagValues.SessionID)

	readConfig := false
	if len(myCommonFlagValues.ConfigFilePath) > 0 {
		// user defined config file
		err := commons.LoadConfigFromFile(myCommonFlagValues.ConfigFilePath)
		if err != nil {
			return false, xerrors.Errorf("failed to load config from file %s: %w", myCommonFlagValues.ConfigFilePath, err) // stop here
		}

		readConfig = true
	} else {
		// read config path from env
		// then read config
		if irodsEnvironmentFileEnvVal, ok := os.LookupEnv(IRODSEnvironmentFileEnvKey); ok {
			if len(irodsEnvironmentFileEnvVal) > 0 {
				err := commons.LoadConfigFromFile(irodsEnvironmentFileEnvVal)
				if err != nil {
					return false, xerrors.Errorf("failed to load config file %s: %w", irodsEnvironmentFileEnvVal, err) // stop here
				}

				readConfig = true
			}
		}

		// read config from default icommands config path
		if !readConfig {
			// auto detect
			err := commons.LoadConfigFromFile("~/.irods")
			if err != nil {
				logger.Debug(err)
				// ignore error
			} else {
				readConfig = true
			}
		}
	}

	// set default config
	if !readConfig {
		commons.SetDefaultConfigIfEmpty()
	}

	err := commons.LoadAndOverwriteConfigFromEnv()
	if err != nil {
		return false, xerrors.Errorf("failed to load config from environment: %w", err) // stop here
	}

	// re-configure level
	if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}

	if retryFlagValues.RetryChild {
		// read from stdin
		err := commons.InputMissingFieldsFromStdin()
		if err != nil {
			return false, xerrors.Errorf("failed to load config from stdin: %w", err) // stop here
		}
	}

	appConfig := commons.GetConfig()

	syncAccount := false
	if myCommonFlagValues.ResourceUpdated {
		appConfig.DefaultResource = myCommonFlagValues.Resource
		logger.Debugf("use default resource server - %s", appConfig.DefaultResource)
		syncAccount = true
	}

	if syncAccount {
		err := commons.SyncAccount()
		if err != nil {
			return false, err
		}
	}

	return true, nil // contiue
}

func printVersion() error {
	info, err := commons.GetVersionJSON()
	if err != nil {
		return xerrors.Errorf("failed to get version json: %w", err)
	}

	fmt.Println(info)
	return nil
}

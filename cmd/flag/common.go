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
	ReadEnvironment bool
	ShowVersion     bool
	ShowHelp        bool
	DebugMode       bool
	logLevelInput   string
	LogLevel        log.Level
	LogLevelUpdated bool
	SessionID       int
	Resource        string
	ResourceUpdated bool
	Ticket          string
	TicketUpdated   bool
}

const (
	IRODSEnvironmentFileEnvKey string = "IRODS_ENVIRONMENT_FILE"
)

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", "", "Set config file or dir (default \"$HOME/.irods\")")
	command.Flags().BoolVarP(&commonFlagValues.ReadEnvironment, "envconfig", "e", false, "Read config from environmental variables")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Set session ID")
	command.Flags().StringVarP(&commonFlagValues.Resource, "resource", "R", "", "Set resource server")
	command.Flags().StringVarP(&commonFlagValues.Ticket, "ticket", "T", "", "Set ticket")

	command.MarkFlagsMutuallyExclusive("debug", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
	command.MarkFlagsMutuallyExclusive("resource", "version")
	command.MarkFlagsMutuallyExclusive("ticket", "version")
	command.MarkFlagsMutuallyExclusive("session", "version")

	command.MarkFlagsMutuallyExclusive("config", "envconfig")
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

	if command.Flags().Changed("ticket") {
		commonFlagValues.TicketUpdated = true
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

	readConfig := false

	if len(myCommonFlagValues.ConfigFilePath) > 0 {
		err := commons.LoadConfigFromFile(myCommonFlagValues.ConfigFilePath)
		if err != nil {
			return false, xerrors.Errorf("failed to load config from file %s: %w", myCommonFlagValues.ConfigFilePath, err) // stop here
		}

		readConfig = true
	}

	if myCommonFlagValues.ReadEnvironment {
		err := commons.LoadConfigFromEnv()
		if err != nil {
			return false, xerrors.Errorf("failed to load config from environment: %w", err) // stop here
		}

		readConfig = true
	}

	// read env config
	if !readConfig {
		if irodsEnvironmentFileEnvVal, ok := os.LookupEnv(IRODSEnvironmentFileEnvKey); ok {
			if len(irodsEnvironmentFileEnvVal) > 0 {
				err := commons.LoadConfigFromFile(irodsEnvironmentFileEnvVal)
				if err != nil {
					return false, xerrors.Errorf("failed to load config file %s: %w", irodsEnvironmentFileEnvVal, err) // stop here
				}

				readConfig = true
			}
		}
	}

	// default config
	if !readConfig {
		// auto detect
		commons.LoadConfigFromFile("~/.irods")
		//if err != nil {
		//logger.Error(err)
		// ignore error
		//}
	}

	commons.SetDefaultConfigIfEmpty()

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

	if myCommonFlagValues.TicketUpdated {
		appConfig.Ticket = myCommonFlagValues.Ticket
		logger.Debugf("use ticket - %s", appConfig.Ticket)
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

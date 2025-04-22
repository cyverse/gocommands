package flag

import (
	"io"
	"os"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

type CommonFlagValues struct {
	ConfigFilePath  string
	ShowVersion     bool
	ShowHelp        bool
	DebugMode       bool
	Quiet           bool
	logLevelInput   string
	LogLevel        log.Level
	LogLevelUpdated bool
	LogFile         string
	LogTerminal     bool
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

func SetCommonFlags(command *cobra.Command, hideResource bool) {
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", commons.GetDefaultIRODSConfigPath(), "Specify custom iRODS configuration file or directory path")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Display version information")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Display help information about available commands and options")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable verbose debug output for troubleshooting")
	command.Flags().BoolVarP(&commonFlagValues.Quiet, "quiet", "q", false, "Suppress all non-error output messages")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG)")
	command.Flags().StringVar(&commonFlagValues.LogFile, "log_file", "", "Specify file path for logging output")
	command.Flags().BoolVarP(&commonFlagValues.LogTerminal, "log_terminal", "", false, "Enable logging to terminal")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Specify session identifier for tracking operations")
	command.Flags().StringVarP(&commonFlagValues.Resource, "resource", "R", "", "Target specific iRODS resource server for operations")

	command.MarkFlagsMutuallyExclusive("quiet", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
	command.MarkFlagsMutuallyExclusive("debug", "quiet", "log_level")

	if !hideResource {
		command.MarkFlagsMutuallyExclusive("resource", "version")
	} else {
		command.Flags().MarkHidden("resource")
	}

	command.MarkFlagsMutuallyExclusive("session", "version")
}

func SetCommonFlagsWithoutResource(command *cobra.Command) {
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", commons.GetDefaultIRODSConfigPath(), "Set config file or directory")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().BoolVarP(&commonFlagValues.Quiet, "quiet", "q", false, "Suppress usual output messages")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")
	command.Flags().StringVar(&commonFlagValues.LogFile, "log_file", "", "Specify file path for logging output")
	command.Flags().BoolVarP(&commonFlagValues.LogTerminal, "log_terminal", "", false, "Enable logging to terminal")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Set session ID")

	command.MarkFlagsMutuallyExclusive("quiet", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
	command.MarkFlagsMutuallyExclusive("debug", "quiet", "log_level")

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

func setLogLevel(command *cobra.Command) {
	myCommonFlagValues := GetCommonFlagValues(command)

	if myCommonFlagValues.Quiet {
		log.SetLevel(log.FatalLevel)
	} else if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}
}

func getLogWriter(logFile string) io.WriteCloser {
	if len(logFile) > 0 {
		return &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    50, // 50MB
			MaxBackups: 5,
			MaxAge:     30, // 30 days
			Compress:   false,
		}
	}

	return nil
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "flag",
		"function": "ProcessCommonFlags",
	})

	myCommonFlagValues := GetCommonFlagValues(command)
	retryFlagValues := GetRetryFlagValues()

	setLogLevel(command)

	if myCommonFlagValues.ShowHelp {
		command.Usage()
		return false, nil // stop here
	}

	if myCommonFlagValues.ShowVersion {
		printVersion()
		return false, nil // stop here
	}

	if len(myCommonFlagValues.LogFile) > 0 {
		fileLogWriter := getLogWriter(myCommonFlagValues.LogFile)

		if myCommonFlagValues.LogTerminal {
			// use multi output - to output to file and stdout
			mw := io.MultiWriter(commons.GetTerminalWriter(), fileLogWriter)
			log.SetOutput(mw)
		} else {
			// use file log writer
			log.SetOutput(fileLogWriter)
		}
	}

	// init config
	err := commons.InitEnvironmentManagerFromSystemConfig()
	if err != nil {
		return false, xerrors.Errorf("failed to init environment manager: %w", err)
	}

	environmentManager := commons.GetEnvironmentManager()

	logger.Debugf("use sessionID - %d", myCommonFlagValues.SessionID)
	environmentManager.SetPPID(myCommonFlagValues.SessionID)

	configFilePath := ""

	// find config file location from env
	if irodsEnvironmentFileEnvVal, ok := os.LookupEnv(IRODSEnvironmentFileEnvKey); ok {
		if len(irodsEnvironmentFileEnvVal) > 0 {
			configFilePath = irodsEnvironmentFileEnvVal
		}
	}

	// user defined config file
	if len(myCommonFlagValues.ConfigFilePath) > 0 {
		configFilePath = myCommonFlagValues.ConfigFilePath
	}

	// load config
	if len(configFilePath) > 0 {
		configFilePath, err = commons.ExpandHomeDir(configFilePath)
		if err != nil {
			return false, xerrors.Errorf("failed to expand home directory for %q: %w", configFilePath, err)
		}

		status, err := os.Stat(configFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				//return false, xerrors.Errorf("config path %q does not exist", configFilePath)
				logger.Debugf("failed to find config path %q as it does not exist", configFilePath)
			} else {
				return false, xerrors.Errorf("failed to stat %q: %w", configFilePath, err)
			}
		} else {
			if status.IsDir() {
				// config root
				err = environmentManager.SetEnvironmentDirPath(configFilePath)
				if err != nil {
					return false, xerrors.Errorf("failed to set configuration root directory %q: %w", configFilePath, err)
				}
			} else {
				// config file
				err = environmentManager.SetEnvironmentFilePath(configFilePath)
				if err != nil {
					return false, xerrors.Errorf("failed to set configuration root directory %q: %w", configFilePath, err)
				}
			}
		}

		// load
		err = environmentManager.Load()
		if err != nil {
			return false, xerrors.Errorf("failed to load configuration file %q: %w", environmentManager.EnvironmentFilePath, err)
		}
	} else {
		// default
		// load
		err = environmentManager.Load()
		if err != nil {
			return false, xerrors.Errorf("failed to load configuration file %q: %w", environmentManager.EnvironmentFilePath, err)
		}
	}

	// load config from env
	envConfig, err := irodsclient_config.NewConfigFromEnv(environmentManager.Environment)
	if err != nil {
		return false, xerrors.Errorf("failed to load config from environment: %w", err)
	}

	// overwrite
	environmentManager.Environment = envConfig

	sessionConfig, err := environmentManager.GetSessionConfig()
	if err != nil {
		return false, xerrors.Errorf("failed to get session config: %w", err)
	}

	if sessionConfig.LogLevel > 0 {
		// set log level
		log.SetLevel(getLogrusLogLevel(sessionConfig.LogLevel))
	}

	// prioritize log level user set via command-line argument
	setLogLevel(command)

	if retryFlagValues.RetryChild {
		// read from stdin
		err := commons.InputMissingFieldsFromStdin()
		if err != nil {
			return false, xerrors.Errorf("failed to load config from stdin: %w", err) // stop here
		}
	}

	if myCommonFlagValues.ResourceUpdated {
		environmentManager.Environment.DefaultResource = myCommonFlagValues.Resource
		logger.Debugf("use default resource server %q", myCommonFlagValues.Resource)
	}

	return true, nil // contiue
}

func printVersion() error {
	info, err := commons.GetVersionJSON()
	if err != nil {
		return xerrors.Errorf("failed to get version json: %w", err)
	}

	commons.Println(info)
	return nil
}

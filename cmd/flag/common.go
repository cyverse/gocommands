package flag

import (
	"io"
	"os"

	"github.com/cockroachdb/errors"
	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"github.com/cyverse/gocommands/commons"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	Timeout         int
	TimeoutUpdated  bool
}

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command, hideResource bool) {
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", config.GetDefaultIRODSConfigPath(), "Specify custom iRODS configuration file or directory path")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Display version information")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Display help information about available commands and options")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable verbose debug output for troubleshooting")
	command.Flags().BoolVarP(&commonFlagValues.Quiet, "quiet", "q", false, "Suppress all non-error output messages")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG)")
	command.Flags().StringVar(&commonFlagValues.LogFile, "log_file", "", "Specify file path for logging output")
	command.Flags().BoolVarP(&commonFlagValues.LogTerminal, "log_terminal", "", false, "Enable logging to terminal")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Specify session identifier for tracking operations")
	command.Flags().StringVarP(&commonFlagValues.Resource, "resource", "R", "", "Target specific iRODS resource server for operations")
	command.Flags().IntVarP(&commonFlagValues.Timeout, "timeout", "", config.GetDefaultFilesystemTimeout(), "Specify timeout duration in seconds")

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
	command.Flags().StringVarP(&commonFlagValues.ConfigFilePath, "config", "c", config.GetDefaultIRODSConfigPath(), "Set config file or directory")
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().BoolVarP(&commonFlagValues.Quiet, "quiet", "q", false, "Suppress usual output messages")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")
	command.Flags().StringVar(&commonFlagValues.LogFile, "log_file", "", "Specify file path for logging output")
	command.Flags().BoolVarP(&commonFlagValues.LogTerminal, "log_terminal", "", false, "Enable logging to terminal")
	command.Flags().IntVarP(&commonFlagValues.SessionID, "session", "s", os.Getppid(), "Set session ID")
	command.Flags().IntVarP(&commonFlagValues.Timeout, "timeout", "", config.GetDefaultFilesystemTimeout(), "Specify timeout duration in seconds")

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

	if command.Flags().Changed("timeout") {
		commonFlagValues.TimeoutUpdated = true
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
		"command": command.Name(),
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
			mw := io.MultiWriter(terminal.GetTerminalWriter(), fileLogWriter)
			log.SetOutput(mw)
		} else {
			// use file log writer
			log.SetOutput(fileLogWriter)
		}
	}

	// init config
	err := config.InitEnvironmentManagerFromSystemConfig()
	if err != nil {
		return false, errors.Wrapf(err, "failed to init environment manager")
	}

	environmentManager := config.GetEnvironmentManager()

	logger.Debugf("use sessionID - %d", myCommonFlagValues.SessionID)
	environmentManager.SetPPID(myCommonFlagValues.SessionID)

	configFilePath := ""

	// user defined config file
	if len(myCommonFlagValues.ConfigFilePath) > 0 {
		configFilePath = myCommonFlagValues.ConfigFilePath
	}

	// load config
	if len(configFilePath) > 0 {
		configFilePath, err = path.ExpandLocalHomeDirPath(configFilePath)
		if err != nil {
			return false, errors.Wrapf(err, "failed to expand home directory for %q", configFilePath)
		}

		status, err := os.Stat(configFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Debugf("failed to find config path %q as it does not exist", configFilePath)
			} else {
				return false, errors.Wrapf(err, "failed to stat %q", configFilePath)
			}
		} else {
			if status.IsDir() {
				// config root
				err = environmentManager.SetEnvironmentDirPath(configFilePath)
				if err != nil {
					return false, errors.Wrapf(err, "failed to set configuration root directory %q", configFilePath)
				}
			} else {
				// config file
				err = environmentManager.SetEnvironmentFilePath(configFilePath)
				if err != nil {
					return false, errors.Wrapf(err, "failed to set configuration root directory %q", configFilePath)
				}
			}
		}

		// load
		err = environmentManager.Load()
		if err != nil {
			return false, errors.Wrapf(err, "failed to load configuration file %q", environmentManager.EnvironmentFilePath)
		}
	} else {
		// default
		// load
		err = environmentManager.Load()
		if err != nil {
			return false, errors.Wrapf(err, "failed to load configuration file %q", environmentManager.EnvironmentFilePath)
		}
	}

	// load config from env
	envConfig, err := irodsclient_config.NewConfigFromEnv(environmentManager.Environment)
	if err != nil {
		return false, errors.Wrapf(err, "failed to load config from environment")
	}

	// overwrite
	environmentManager.Environment = envConfig

	sessionConfig, err := environmentManager.GetSessionConfig()
	if err != nil {
		return false, errors.Wrapf(err, "failed to get session config")
	}

	if sessionConfig.LogLevel > 0 {
		// set log level
		log.SetLevel(getLogrusLogLevel(sessionConfig.LogLevel))
	}

	// prioritize log level user set via command-line argument
	setLogLevel(command)

	if retryFlagValues.RetryChild {
		// read from stdin
		err := config.InputMissingFieldsFromStdin()
		if err != nil {
			return false, errors.Wrapf(err, "failed to load config from stdin")
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
		return errors.Wrapf(err, "failed to get version json")
	}

	terminal.Println(info)
	return nil
}

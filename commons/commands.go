package commons

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().StringP("config", "c", "", "config file (default is $HOME/.irods/irods_environment.json)")
	command.Flags().BoolP("version", "v", false, "Print version")
	command.Flags().BoolP("help", "h", false, "Print help")
}

func ProcessCommonFlags(command *cobra.Command) {
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
			PrintHelp(command)
			return
		}
	}

	versionFlag := command.Flags().Lookup("version")
	if versionFlag != nil {
		version, err := strconv.ParseBool(versionFlag.Value.String())
		if err != nil {
			version = false
		}

		if version {
			PrintVersion(command)
			return
		}
	}

	configFlag := command.Flags().Lookup("config")
	if configFlag != nil {
		config, err := strconv.ParseBool(configFlag.Value.String())
		if err != nil {
			config = false
		}

		if config {
			logger.Debugf("reading config - %s", config)
			// TODO
			LoadConfigFile(command)
		}
	}
}

func LoadConfigFile(command *cobra.Command) {

}

func PrintVersion(command *cobra.Command) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "PrintVersion",
	})

	info, err := GetVersionJSON()
	if err != nil {
		logger.WithError(err).Error("failed to get client version info")
		return err
	}

	fmt.Println(info)
	return nil
}

func PrintHelp(command *cobra.Command) error {
	return command.Usage()
}

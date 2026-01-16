package subcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"
)

var envsCmd = &cobra.Command{
	Use:     "envs",
	Aliases: []string{"ienvs"},
	Short:   "Print all available iRODS environments",
	Long:    `This command prints all available iRODS environments.`,
	RunE:    processEnvsCommand,
	Args:    cobra.NoArgs,
}

func AddEnvsCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(envsCmd, true)
	flag.SetOutputFormatFlags(envsCmd)

	rootCmd.AddCommand(envsCmd)
}

func processEnvsCommand(command *cobra.Command, args []string) error {
	envs, err := NewEnvsCommand(command, args)
	if err != nil {
		return err
	}

	return envs.Process()
}

type EnvsCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
}

func NewEnvsCommand(command *cobra.Command, args []string) (*EnvsCommand, error) {
	envs := &EnvsCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
	}

	return envs, nil
}

func (envs *EnvsCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(envs.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = envs.printEnvironments()
	if err != nil {
		return errors.Wrapf(err, "failed to print environment")
	}

	return nil
}

func (envs *EnvsCommand) printEnvironments() error {
	envMgr := config.GetEnvironmentManager()
	if envMgr == nil {
		return errors.Errorf("environment is not set")
	}

	dirPath := envMgr.EnvironmentDirPath

	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())
	outputFormatterTable := outputFormatter.NewTable("Available iRODS Environments")

	envFiles, err := os.ReadDir(dirPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read environment directory %q", dirPath)
	}

	envFileNames := []string{}
	for _, envFile := range envFiles {
		if !envFile.IsDir() && envs.isTargetEnvFile(envFile.Name()) {
			// environment file
			envFileNames = append(envFileNames, envFile.Name())
		}
	}

	outputFormatterTable.SetHeader([]string{
		"Environment Name",
		"Environment File Path",
		"Host",
		"Port",
		"Zone",
		"Home",
		"Authentication Scheme",
	})

	for _, envFileName := range envFileNames {
		envFilePath := filepath.Join(dirPath, envFileName)

		manager, err := irodsclient_config.NewICommandsEnvironmentManager()
		if err != nil {
			return errors.Wrapf(err, "failed to create a new environment manager")
		}

		err = manager.SetEnvironmentFilePath(envFilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to set environment file path %q", envFilePath)
		}

		err = manager.Load()
		if err != nil {
			// return errors.Wrapf(err, "failed to load environment file %q", envFilePath)
			// failed to load - skip
			continue
		}

		sessionConfig, err := manager.GetSessionConfig()
		if err != nil {
			//  return errors.Wrapf(err, "failed to get session config from environment file %q", envFilePath)
			// failed to get session config - skip
			continue
		}

		environmentName := envs.getEnvName(envFileName)
		if envFilePath == irodsclient_config.GetDefaultEnvironmentFilePath() {
			environmentName += " (current)"
		}

		outputFormatterTable.AppendRow([]interface{}{
			environmentName,
			envFilePath,
			sessionConfig.Host,
			sessionConfig.Port,
			sessionConfig.ZoneName,
			sessionConfig.Username,
			sessionConfig.AuthenticationScheme,
		})
	}

	outputFormatter.Render(envs.outputFormatFlagValues.Format)

	return nil
}

func (envs *EnvsCommand) isTargetEnvFile(p string) bool {
	return strings.HasSuffix(p, ".env.json")
}

func (envs *EnvsCommand) getEnvName(p string) string {
	return strings.TrimSuffix(p, ".env.json")
}

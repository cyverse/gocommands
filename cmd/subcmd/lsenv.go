package subcmd

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/format"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/spf13/cobra"
)

var lsenvCmd = &cobra.Command{
	Use:     "lsenv",
	Aliases: []string{"ienvs", "envs"},
	Short:   "Print all available iRODS environments",
	Long:    `This command prints all available iRODS environments.`,
	RunE:    processLsenvCommand,
	Args:    cobra.NoArgs,
}

func AddLsenvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(lsenvCmd, true)
	flag.SetOutputFormatFlags(lsenvCmd)

	rootCmd.AddCommand(lsenvCmd)
}

func processLsenvCommand(command *cobra.Command, args []string) error {
	lsenv, err := NewLsenvCommand(command, args)
	if err != nil {
		return err
	}

	return lsenv.Process()
}

type LsenvCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	outputFormatFlagValues *flag.OutputFormatFlagValues
}

func NewLsenvCommand(command *cobra.Command, args []string) (*LsenvCommand, error) {
	lsenv := &LsenvCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		outputFormatFlagValues: flag.GetOutputFormatFlagValues(),
	}

	return lsenv, nil
}

func (lsenv *LsenvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(lsenv.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = lsenv.printEnvironments()
	if err != nil {
		return errors.Wrapf(err, "failed to print environment")
	}

	return nil
}

func (lsenv *LsenvCommand) printEnvironments() error {
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
		if !envFile.IsDir() && config.IsTargetEnvFile(envFile.Name()) {
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

		environmentName := config.GetEnvName(envFileName)
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

	outputFormatter.Render(lsenv.outputFormatFlagValues.Format)

	return nil
}

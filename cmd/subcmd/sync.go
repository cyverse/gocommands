package subcmd

import (
	"os"
	"strings"

	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var syncCmd = &cobra.Command{
	Use:     "sync i:[collection] [local dir] or sync [local dir] i:[collection]",
	Aliases: []string{"isync"},
	Short:   "Sync local directory with iRODS collection",
	Long:    `This synchronizes a local directory with the given iRODS collection.`,
	RunE:    processSyncCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddSyncCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(syncCmd)

	flag.SetBundleTempFlags(syncCmd)
	flag.SetBundleClearFlags(syncCmd)
	flag.SetBundleConfigFlags(syncCmd)
	flag.SetParallelTransferFlags(syncCmd, true)
	flag.SetForceFlags(syncCmd, true)
	flag.SetProgressFlags(syncCmd)
	flag.SetRetryFlags(syncCmd)
	flag.SetDifferentialTransferFlags(syncCmd, false)

	rootCmd.AddCommand(syncCmd)
}

func processSyncCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	retryFlagValues := flag.GetRetryFlagValues()

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	targetPath := args[len(args)-1]
	sourcePaths := args[:len(args)-1]

	localSources := []string{}
	irodsSources := []string{}

	for _, sourcePath := range sourcePaths {
		if strings.HasPrefix(sourcePath, "i:") {
			irodsSources = append(irodsSources, sourcePath[2:])
		} else {
			localSources = append(localSources, sourcePath)
		}
	}

	if len(localSources) > 0 {
		// source is local
		if !strings.HasPrefix(targetPath, "i:") {
			// local to local
			return xerrors.Errorf("syncing local to local is not supported")
		}

		// local to iRODS
		// target must starts with "i:"
		err := syncFromLocalToIRODS(command)
		if err != nil {
			return xerrors.Errorf("failed to perform sync (from local to iRODS): %w", err)
		}
	}

	if len(irodsSources) > 0 {
		// source is iRODS
		if strings.HasPrefix(targetPath, "i:") {
			// iRODS to iRODS
			err := syncFromIRODSToIRODS(command, irodsSources, targetPath[2:])
			if err != nil {
				return xerrors.Errorf("failed to perform sync (from iRODS to iRODS): %w", err)
			}
		} else {
			// iRODS to local
			err := syncFromIRODSToLocal(command, irodsSources, targetPath)
			if err != nil {
				return xerrors.Errorf("failed to perform sync (from iRODS to local): %w", err)
			}
		}
	}

	return nil
}

func syncFromLocalToIRODS(command *cobra.Command) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncFromLocalToIRODS",
	})

	newArgs := []string{}

	commandName := command.CalledAs()
	commandIdx := -1

	osArgs := os.Args[1:]
	for argIdx, arg := range osArgs {
		if arg == commandName {
			commandIdx = argIdx
			break
		}
	}

	if commandIdx < 0 {
		return xerrors.Errorf("failed to find command location")
	}

	newArgs = append(newArgs, osArgs[:commandIdx]...)
	newArgs = append(newArgs, "--diff")
	newArgs = append(newArgs, osArgs[commandIdx+1:]...)

	// filter out retry flag
	newArgs2 := []string{}
	for _, arg := range newArgs {
		if arg != "--retry_child" {
			newArgs2 = append(newArgs2, arg)
		}
	}

	// run bput
	logger.Debugf("run bput with args: %v", newArgs2)
	bputCmd.ParseFlags(newArgs2)
	argWoFlags := bputCmd.Flags().Args()
	return bputCmd.RunE(bputCmd, argWoFlags)
}

func syncFromIRODSToIRODS(command *cobra.Command, sourcePaths []string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncFromIRODSToIRODS",
	})

	newArgs := []string{}

	commandName := command.CalledAs()
	commandIdx := -1

	osArgs := os.Args[1:]
	for argIdx, arg := range osArgs {
		if arg == commandName {
			commandIdx = argIdx
			break
		}
	}

	if commandIdx < 0 {
		return xerrors.Errorf("failed to find command location")
	}

	newArgs = append(newArgs, osArgs[:commandIdx]...)
	newArgs = append(newArgs, "--diff")
	newArgs = append(newArgs, osArgs[commandIdx+1:]...)

	// filter out retry flag
	newArgs2 := []string{}
	for _, arg := range newArgs {
		if arg != "--retry_child" {
			newArgs2 = append(newArgs2, arg)
		}
	}

	// run bput
	logger.Debugf("run cp with args: %v", newArgs2)
	cpCmd.ParseFlags(newArgs2)
	argWoFlags := cpCmd.Flags().Args()
	return cpCmd.RunE(cpCmd, argWoFlags)
}

func syncFromIRODSToLocal(command *cobra.Command, sourcePaths []string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncFromIRODSToLocal",
	})

	newArgs := []string{}

	commandName := command.CalledAs()
	commandIdx := -1

	osArgs := os.Args[1:]
	for argIdx, arg := range osArgs {
		if arg == commandName {
			commandIdx = argIdx
			break
		}
	}

	if commandIdx < 0 {
		return xerrors.Errorf("failed to find command location")
	}

	newArgs = append(newArgs, osArgs[:commandIdx]...)
	newArgs = append(newArgs, "--diff")
	newArgs = append(newArgs, osArgs[commandIdx+1:]...)

	// filter out retry flag
	newArgs2 := []string{}
	for _, arg := range newArgs {
		if arg != "--retry_child" {
			newArgs2 = append(newArgs2, arg)
		}
	}

	// run bput
	logger.Debugf("run get with args: %v", newArgs2)
	getCmd.ParseFlags(newArgs2)
	argWoFlags := getCmd.Flags().Args()
	return getCmd.RunE(getCmd, argWoFlags)
}

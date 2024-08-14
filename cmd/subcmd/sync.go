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
	flag.SetCommonFlags(syncCmd, false)

	flag.SetBundleTransferFlags(syncCmd, true)
	flag.SetParallelTransferFlags(syncCmd, true)
	flag.SetForceFlags(syncCmd, true)
	flag.SetProgressFlags(syncCmd)
	flag.SetRetryFlags(syncCmd)
	flag.SetDifferentialTransferFlags(syncCmd, false)
	flag.SetNoRootFlags(syncCmd)
	flag.SetSyncFlags(syncCmd)

	rootCmd.AddCommand(syncCmd)
}

func processSyncCommand(command *cobra.Command, args []string) error {
	sync, err := NewSyncCommand(command, args)
	if err != nil {
		return err
	}

	return sync.Process()
}

type SyncCommand struct {
	command *cobra.Command

	retryFlagValues *flag.RetryFlagValues

	sourcePaths []string
	targetPath  string
}

func NewSyncCommand(command *cobra.Command, args []string) (*SyncCommand, error) {
	sync := &SyncCommand{
		command: command,

		retryFlagValues: flag.GetRetryFlagValues(),
	}

	// path
	sync.sourcePaths = args[:len(args)-1]
	sync.targetPath = args[len(args)-1]

	return sync, nil
}

func (sync *SyncCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(sync.command)
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

	// handle retry
	if sync.retryFlagValues.RetryNumber > 0 && !sync.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(sync.retryFlagValues.RetryNumber, sync.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", sync.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	localSourcePaths := []string{}
	irodsSourcePaths := []string{}

	for _, sourcePath := range sync.sourcePaths {
		if strings.HasPrefix(sourcePath, "i:") {
			// irods
			irodsSourcePaths = append(irodsSourcePaths, sourcePath[2:])
		} else {
			// local
			localSourcePaths = append(localSourcePaths, sourcePath)
		}
	}

	if len(localSourcePaths) > 0 {
		err := sync.syncLocal(sync.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to sync (from local): %w", err)
		}
	}

	if len(irodsSourcePaths) > 0 {
		err := sync.syncIRODS(sync.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to sync (from iRODS): %w", err)
		}
	}

	return nil
}

func (sync *SyncCommand) syncLocal(targetPath string) error {
	if !strings.HasPrefix(targetPath, "i:") {
		// local to local
		return xerrors.Errorf("syncing local to local is not supported")
	}

	// local to iRODS
	// target must starts with "i:"
	err := sync.syncLocalToIRODS()
	if err != nil {
		return xerrors.Errorf("failed to sync (from local to iRODS): %w", err)
	}

	return nil
}

func (sync *SyncCommand) syncIRODS(targetPath string) error {
	if strings.HasPrefix(targetPath, "i:") {
		// iRODS to iRODS
		err := sync.syncIRODSToIRODS()
		if err != nil {
			return xerrors.Errorf("failed to sync (from iRODS to iRODS): %w", err)
		}

		return nil
	}

	// iRODS to local
	err := sync.syncIRODSToLocal()
	if err != nil {
		return xerrors.Errorf("failed to sync (from iRODS to local): %w", err)
	}

	return nil
}

func (sync *SyncCommand) getNewCommandArgsForRetry() ([]string, error) {
	newArgs := []string{}

	commandName := sync.command.CalledAs()
	commandIdx := -1

	osArgs := os.Args[1:]
	for argIdx, arg := range osArgs {
		if arg == commandName {
			commandIdx = argIdx
			break
		}
	}

	if commandIdx < 0 {
		return nil, xerrors.Errorf("failed to find command location")
	}

	newArgs = append(newArgs, osArgs[:commandIdx]...)
	newArgs = append(newArgs, "--diff")
	newArgs = append(newArgs, osArgs[commandIdx+1:]...)

	// filter out retry flag
	newArgsWoRetryFlag := []string{}
	for _, arg := range newArgs {
		if arg != "--retry_child" {
			newArgsWoRetryFlag = append(newArgsWoRetryFlag, arg)
		}
	}

	return newArgsWoRetryFlag, nil
}

func (sync *SyncCommand) syncLocalToIRODS() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SyncCommand",
		"function": "syncLocalToIRODS",
	})

	newArgs, err := sync.getNewCommandArgsForRetry()
	if err != nil {
		return xerrors.Errorf("failed to get new command args for retry: %w", err)
	}

	// run bput
	logger.Debugf("run bput with args: %v", newArgs)
	bputCmd.ParseFlags(newArgs)
	argWoFlags := bputCmd.Flags().Args()
	return bputCmd.RunE(bputCmd, argWoFlags)
}

func (sync *SyncCommand) syncIRODSToIRODS() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SyncCommand",
		"function": "syncIRODSToIRODS",
	})

	newArgs, err := sync.getNewCommandArgsForRetry()
	if err != nil {
		return xerrors.Errorf("failed to get new command args for retry: %w", err)
	}

	// run cp
	logger.Debugf("run cp with args: %v", newArgs)
	cpCmd.ParseFlags(newArgs)
	argWoFlags := cpCmd.Flags().Args()
	return cpCmd.RunE(cpCmd, argWoFlags)
}

func (sync *SyncCommand) syncIRODSToLocal() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SyncCommand",
		"function": "syncIRODSToLocal",
	})

	newArgs, err := sync.getNewCommandArgsForRetry()
	if err != nil {
		return xerrors.Errorf("failed to get new command args for retry: %w", err)
	}

	// run get
	logger.Debugf("run get with args: %v", newArgs)
	getCmd.ParseFlags(newArgs)
	argWoFlags := getCmd.Flags().Args()
	return getCmd.RunE(getCmd, argWoFlags)
}

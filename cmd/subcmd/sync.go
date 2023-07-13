package subcmd

import (
	"strings"

	"github.com/cyverse/gocommands/cmd/flag"
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
		err := syncFromLocalToIRODS(command, localSources, targetPath[2:])
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

func syncFromLocalToIRODS(command *cobra.Command, sourcePaths []string, targetPath string) error {
	newArgs := []string{}
	newArgs = append(newArgs, sourcePaths...)
	newArgs = append(newArgs, targetPath)

	// pass to bput
	return processBputCommand(command, newArgs)
}

func syncFromIRODSToIRODS(command *cobra.Command, sourcePaths []string, targetPath string) error {
	newArgs := []string{}
	newArgs = append(newArgs, sourcePaths...)
	newArgs = append(newArgs, targetPath)

	// pass to cp
	return processCpCommand(command, newArgs)
}

func syncFromIRODSToLocal(command *cobra.Command, sourcePaths []string, targetPath string) error {
	newArgs := []string{}
	newArgs = append(newArgs, sourcePaths...)
	newArgs = append(newArgs, targetPath)

	// pass to get
	return processGetCommand(command, newArgs)
}

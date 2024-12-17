package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rmCmd = &cobra.Command{
	Use:     "rm [data-object1] [data-object2] [collection1] ...",
	Aliases: []string{"irm", "del", "remove"},
	Short:   "Remove iRODS data-objects or collections",
	Long:    `This removes iRODS data-objects or collections.`,
	RunE:    processRmCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmCmd, false)

	flag.SetForceFlags(rmCmd, false)
	flag.SetRecursiveFlags(rmCmd, false)

	rootCmd.AddCommand(rmCmd)
}

func processRmCommand(command *cobra.Command, args []string) error {
	rm, err := NewRmCommand(command, args)
	if err != nil {
		return err
	}

	return rm.Process()
}

type RmCommand struct {
	command *cobra.Command

	commonFlagValues    *flag.CommonFlagValues
	forceFlagValues     *flag.ForceFlagValues
	recursiveFlagValues *flag.RecursiveFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewRmCommand(command *cobra.Command, args []string) (*RmCommand, error) {
	rm := &RmCommand{
		command: command,

		commonFlagValues:    flag.GetCommonFlagValues(command),
		forceFlagValues:     flag.GetForceFlagValues(),
		recursiveFlagValues: flag.GetRecursiveFlagValues(),
	}

	// path
	rm.targetPaths = args

	return rm, nil
}

func (rm *RmCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rm.command)
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

	// Create a file system
	rm.account = commons.GetSessionConfig().ToIRODSAccount()
	rm.filesystem, err = commons.GetIRODSFSClientForSingleOperation(rm.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer rm.filesystem.Release()

	// remove
	for _, targetPath := range rm.targetPaths {
		err = rm.removeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to remove %q: %w", targetPath, err)
		}
	}
	return nil
}

func (rm *RmCommand) removeOne(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmCommand",
		"function": "removeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := rm.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := rm.filesystem.Stat(targetPath)
	if err != nil {
		logger.Debugf("failed to find a data object %q, but trying to remove", targetPath)
		err = rm.filesystem.RemoveFile(targetPath, rm.forceFlagValues.Force)
		if err != nil {
			return xerrors.Errorf("failed to remove %q: %w", targetPath, err)
		}
		return nil
	}

	if targetEntry.IsDir() {
		// dir
		if !rm.recursiveFlagValues.Recursive {
			return xerrors.Errorf("cannot remove a collection, recurse is not set")
		}

		logger.Debugf("removing a collection %q", targetPath)
		err = rm.filesystem.RemoveDir(targetPath, rm.recursiveFlagValues.Recursive, rm.forceFlagValues.Force)
		if err != nil {
			return xerrors.Errorf("failed to remove a directory %q: %w", targetPath, err)
		}

		return nil
	}

	// file
	logger.Debugf("removing a data object %q", targetPath)
	err = rm.filesystem.RemoveFile(targetPath, rm.forceFlagValues.Force)
	if err != nil {
		return xerrors.Errorf("failed to remove %q: %w", targetPath, err)
	}

	return nil
}

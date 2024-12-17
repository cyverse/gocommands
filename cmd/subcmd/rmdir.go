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

var rmdirCmd = &cobra.Command{
	Use:     "rmdir [collection1] [collection2] ...",
	Aliases: []string{"irmdir"},
	Short:   "Remove iRODS collections",
	Long:    `This removes iRODS collections.`,
	RunE:    processRmdirCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmdirCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmdirCmd, false)

	flag.SetForceFlags(rmdirCmd, false)
	flag.SetRecursiveFlags(rmdirCmd, false)

	rootCmd.AddCommand(rmdirCmd)
}

func processRmdirCommand(command *cobra.Command, args []string) error {
	rm, err := NewRmDirCommand(command, args)
	if err != nil {
		return err
	}

	return rm.Process()
}

type RmDirCommand struct {
	command *cobra.Command

	commonFlagValues    *flag.CommonFlagValues
	forceFlagValues     *flag.ForceFlagValues
	recursiveFlagValues *flag.RecursiveFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewRmDirCommand(command *cobra.Command, args []string) (*RmDirCommand, error) {
	rmDir := &RmDirCommand{
		command: command,

		commonFlagValues:    flag.GetCommonFlagValues(command),
		forceFlagValues:     flag.GetForceFlagValues(),
		recursiveFlagValues: flag.GetRecursiveFlagValues(),
	}

	// path
	rmDir.targetPaths = args

	return rmDir, nil
}

func (rmDir *RmDirCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rmDir.command)
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
	rmDir.account = commons.GetSessionConfig().ToIRODSAccount()
	rmDir.filesystem, err = commons.GetIRODSFSClientForSingleOperation(rmDir.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer rmDir.filesystem.Release()

	// rmdir
	for _, targetPath := range rmDir.targetPaths {
		err = rmDir.removeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to remove a directory %q: %w", targetPath, err)
		}
	}

	return nil
}

func (rmDir *RmDirCommand) removeOne(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmDirCommand",
		"function": "removeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := rmDir.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := rmDir.filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		// file
		return commons.NewNotDirError(targetPath)
	}

	// dir
	logger.Debugf("removing a directory %q", targetPath)
	err = rmDir.filesystem.RemoveDir(targetPath, rmDir.recursiveFlagValues.Recursive, rmDir.forceFlagValues.Force)
	if err != nil {
		return xerrors.Errorf("failed to remove a directory %q: %w", targetPath, err)
	}

	return nil
}

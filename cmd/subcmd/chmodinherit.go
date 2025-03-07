package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var chmodinheritCmd = &cobra.Command{
	Use:     "chmodinherit [inherit | noinherit] [data-object or collection]",
	Aliases: []string{"ch_mod_inherit", "ch_inherit", "change_inherit", "change_mod_inherit", "modify_inherit", "modify_mod_inherit", "update_inherit", "update_mod_inherit"},
	Short:   "Modify access inherit",
	Long:    `This modifies access inherit to data-objects or collections.`,
	RunE:    processChmodinheritCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddChmodinheritCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(chmodinheritCmd, true)

	flag.SetRecursiveFlags(chmodinheritCmd, false)

	rootCmd.AddCommand(chmodinheritCmd)
}

func processChmodinheritCommand(command *cobra.Command, args []string) error {
	chInherit, err := NewChModInheritCommand(command, args)
	if err != nil {
		return err
	}

	return chInherit.Process()
}

type ChModInheritCommand struct {
	command *cobra.Command

	commonFlagValues    *flag.CommonFlagValues
	recursiveFlagValues *flag.RecursiveFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	inherit     bool
	targetPaths []string
}

func NewChModInheritCommand(command *cobra.Command, args []string) (*ChModInheritCommand, error) {
	chModInherit := &ChModInheritCommand{
		command: command,

		commonFlagValues:    flag.GetCommonFlagValues(command),
		recursiveFlagValues: flag.GetRecursiveFlagValues(),
	}

	if len(args) >= 2 {
		chModInherit.targetPaths = args[1:]

		if args[0] == "inherit" {
			chModInherit.inherit = true
		} else if args[0] == "noinherit" || args[0] == "no_inherit" || args[0] == "no-inherit" {
			chModInherit.inherit = false
		} else {
			return nil, xerrors.Errorf("invalid inherit flag: %s", args[0])
		}
	}

	return chModInherit, nil
}

func (chModInherit *ChModInheritCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(chModInherit.command)
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
	chModInherit.account = commons.GetSessionConfig().ToIRODSAccount()
	chModInherit.filesystem, err = commons.GetIRODSFSClientForSingleOperation(chModInherit.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer chModInherit.filesystem.Release()

	for _, targetPath := range chModInherit.targetPaths {
		err = chModInherit.changeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to change access inherit to %q: %w", targetPath, err)
		}
	}

	return nil
}

func (chModInherit *ChModInheritCommand) changeOne(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := chModInherit.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := chModInherit.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to find collection %q: %w", targetPath, err)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetEntry.IsDir() {
		return xerrors.Errorf("target %q is not a collection", targetPath)
	}

	conn, err := chModInherit.filesystem.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get metadata connection: %w", err)
	}
	defer chModInherit.filesystem.ReturnMetadataConnection(conn)

	err = irodsclient_irodsfs.ChangeAccessInherit(conn, targetPath, chModInherit.inherit, chModInherit.recursiveFlagValues.Recursive, false)
	if err != nil {
		return xerrors.Errorf("failed to set access inherit %t to %q (recurse: %t): %w", chModInherit.inherit, targetPath, chModInherit.recursiveFlagValues.Recursive, err)
	}

	return nil
}

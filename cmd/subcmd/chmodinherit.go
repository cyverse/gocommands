package subcmd

import (
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var chmodinheritCmd = &cobra.Command{
	Use:     "chmodinherit <inherit|noinherit> <collection>",
	Aliases: []string{"ch_mod_inherit", "ch_inherit", "change_inherit", "change_mod_inherit", "modify_inherit", "modify_mod_inherit", "update_inherit", "update_mod_inherit"},
	Short:   "Modify access inheritance for iRODS collections",
	Long:    `This command modifies the access inheritance setting for the specified iRODS collections.`,
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

		flag := strings.ToLower(args[0])

		if flag == "inherit" || flag == "true" || flag == "yes" || flag == "enabled" || flag == "on" || flag == "enable" {
			chModInherit.inherit = true
		} else if flag == "noinherit" || flag == "no_inherit" || flag == "no-inherit" || flag == "false" || flag == "no" || flag == "disabled" || flag == "off" || flag == "disable" {
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
	_, err = config.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	chModInherit.account = config.GetSessionConfig().ToIRODSAccount()
	chModInherit.filesystem, err = irods.GetIRODSFSClient(chModInherit.account, true, true)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer chModInherit.filesystem.Release()

	if chModInherit.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(chModInherit.filesystem, chModInherit.commonFlagValues.Timeout)
	}

	for _, targetPath := range chModInherit.targetPaths {
		err = chModInherit.changeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to change access inherit to %q: %w", targetPath, err)
		}
	}

	return nil
}

func (chModInherit *ChModInheritCommand) changeOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := chModInherit.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

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

	err = chModInherit.filesystem.ChangeDirACLInheritance(targetPath, chModInherit.inherit, chModInherit.recursiveFlagValues.Recursive, false)
	if err != nil {
		return xerrors.Errorf("failed to set access inherit %t to %q (recurse: %t): %w", chModInherit.inherit, targetPath, chModInherit.recursiveFlagValues.Recursive, err)
	}

	return nil
}

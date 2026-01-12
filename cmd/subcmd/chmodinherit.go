package subcmd

import (
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/spf13/cobra"
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
			return nil, errors.Errorf("invalid inherit flag %q", args[0])
		}
	}

	return chModInherit, nil
}

func (chModInherit *ChModInheritCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(chModInherit.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// Create a file system
	chModInherit.account = config.GetSessionConfig().ToIRODSAccount()
	chModInherit.filesystem, err = irods.GetIRODSFSClient(chModInherit.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer chModInherit.filesystem.Release()

	if chModInherit.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(chModInherit.filesystem, chModInherit.commonFlagValues.Timeout)
	}

	for _, targetPath := range chModInherit.targetPaths {
		err = chModInherit.changeOne(targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to change access inherit to %q", targetPath)
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
			return errors.Wrapf(err, "failed to find collection %q", targetPath)
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if !targetEntry.IsDir() {
		return errors.Errorf("target %q is not a collection", targetPath)
	}

	err = chModInherit.filesystem.ChangeDirACLInheritance(targetPath, chModInherit.inherit, chModInherit.recursiveFlagValues.Recursive, false)
	if err != nil {
		return errors.Wrapf(err, "failed to set access inherit %t to %q (recurse %t)", chModInherit.inherit, targetPath, chModInherit.recursiveFlagValues.Recursive)
	}

	return nil
}

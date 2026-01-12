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

var chmodCmd = &cobra.Command{
	Use:     "chmod <access-level> <user-or-group(#zone)> <data-object-or-collection>",
	Aliases: []string{"ichmod", "ch_mod", "change_mod", "update_mod", "ch_access", "change_access", "update_access"},
	Short:   "Modify access to iRODS data objects or collections",
	Long:    `This command modifies access permissions for the specified iRODS data objects or collections.`,
	RunE:    processChmodCommand,
	Args:    cobra.MinimumNArgs(3),
}

func AddChmodCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(chmodCmd, true)

	flag.SetRecursiveFlags(chmodCmd, false)

	rootCmd.AddCommand(chmodCmd)
}

func processChmodCommand(command *cobra.Command, args []string) error {
	chMod, err := NewChModCommand(command, args)
	if err != nil {
		return err
	}

	return chMod.Process()
}

type ChModCommand struct {
	command *cobra.Command

	commonFlagValues    *flag.CommonFlagValues
	recursiveFlagValues *flag.RecursiveFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	accessLevel irodsclient_types.IRODSAccessLevelType
	username    string
	zoneName    string
	targetPaths []string
}

func NewChModCommand(command *cobra.Command, args []string) (*ChModCommand, error) {
	chMod := &ChModCommand{
		command: command,

		commonFlagValues:    flag.GetCommonFlagValues(command),
		recursiveFlagValues: flag.GetRecursiveFlagValues(),
	}

	if len(args) >= 3 {
		chMod.accessLevel = irodsclient_types.GetIRODSAccessLevelType(args[0])
		if strings.Contains(args[1], "#") {
			parts := strings.Split(args[1], "#")
			chMod.username = parts[0]
			chMod.zoneName = parts[1]
		} else {
			chMod.username = args[1]
			chMod.zoneName = ""
		}
		chMod.targetPaths = args[2:]
	}

	return chMod, nil
}

func (chMod *ChModCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(chMod.command)
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
	chMod.account = config.GetSessionConfig().ToIRODSAccount()
	chMod.filesystem, err = irods.GetIRODSFSClient(chMod.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer chMod.filesystem.Release()

	if chMod.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(chMod.filesystem, chMod.commonFlagValues.Timeout)
	}

	for _, targetPath := range chMod.targetPaths {
		err = chMod.changeOne(targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to change access to %q", targetPath)
		}
	}

	return nil
}

func (chMod *ChModCommand) changeOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := chMod.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	_, err := chMod.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			return errors.Wrapf(err, "failed to find data-object/collection %q", targetPath)
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	zoneName := chMod.zoneName
	if len(zoneName) == 0 {
		zoneName = chMod.account.ClientZone
	}

	err = chMod.filesystem.ChangeACLs(targetPath, chMod.accessLevel, chMod.username, zoneName, chMod.recursiveFlagValues.Recursive, false)
	if err != nil {
		return errors.Wrapf(err, "failed to change access to %q", targetPath)
	}

	return nil
}

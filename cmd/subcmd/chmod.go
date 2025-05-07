package subcmd

import (
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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
	chMod.account = commons.GetSessionConfig().ToIRODSAccount()
	chMod.filesystem, err = commons.GetIRODSFSClientForLongSingleOperation(chMod.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer chMod.filesystem.Release()

	for _, targetPath := range chMod.targetPaths {
		err = chMod.changeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to change access to %q: %w", targetPath, err)
		}
	}

	return nil
}

func (chMod *ChModCommand) changeOne(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := chMod.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	_, err := chMod.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to find data-object/collection %q: %w", targetPath, err)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	zoneName := chMod.zoneName
	if len(zoneName) == 0 {
		zoneName = chMod.account.ClientZone
	}

	err = chMod.filesystem.ChangeACLs(targetPath, chMod.accessLevel, chMod.username, zoneName, chMod.recursiveFlagValues.Recursive, false)
	if err != nil {
		return xerrors.Errorf("failed to change access: %w", err)
	}

	return nil
}

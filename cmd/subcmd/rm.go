package subcmd

import (
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/wildcard"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm <data-object-or-collection>...",
	Aliases: []string{"irm", "del", "remove"},
	Short:   "Remove iRODS data-objects or collections",
	Long:    `This command removes iRODS data-objects or collections.`,
	RunE:    processRmCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmCmd, false)

	flag.SetForceFlags(rmCmd, false)
	flag.SetRecursiveFlags(rmCmd, false)
	flag.SetWildcardSearchFlags(rmCmd)

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

	commonFlagValues         *flag.CommonFlagValues
	recursiveFlagValues      *flag.RecursiveFlagValues
	forceFlagValues          *flag.ForceFlagValues
	wildcardSearchFlagValues *flag.WildcardSearchFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewRmCommand(command *cobra.Command, args []string) (*RmCommand, error) {
	rm := &RmCommand{
		command: command,

		commonFlagValues:         flag.GetCommonFlagValues(command),
		recursiveFlagValues:      flag.GetRecursiveFlagValues(),
		forceFlagValues:          flag.GetForceFlagValues(),
		wildcardSearchFlagValues: flag.GetWildcardSearchFlagValues(),
	}

	// path
	rm.targetPaths = args

	return rm, nil
}

func (rm *RmCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rm.command)
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
	rm.account = config.GetSessionConfig().ToIRODSAccount()
	rm.filesystem, err = irods.GetIRODSFSClient(rm.account, true, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer rm.filesystem.Release()

	if rm.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(rm.filesystem, rm.commonFlagValues.Timeout)
	}

	// Expand wildcards
	if rm.wildcardSearchFlagValues.WildcardSearch {
		rm.targetPaths, err = wildcard.ExpandWildcards(rm.filesystem, rm.account, rm.targetPaths, true, true)
		if err != nil {
			return errors.Wrapf(err, "failed to expand wildcards")
		}
	}

	// remove
	for _, targetPath := range rm.targetPaths {
		err = rm.removeOne(targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to remove %q", targetPath)
		}
	}
	return nil
}

func (rm *RmCommand) removeOne(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
	})

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := rm.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := rm.filesystem.Stat(targetPath)
	if err != nil {
		logger.Debug("failed to find a data object, but trying to remove")
		err = rm.filesystem.RemoveFile(targetPath, rm.forceFlagValues.Force)
		if err != nil {
			return errors.Wrapf(err, "failed to remove %q", targetPath)
		}
		return nil
	}

	if targetEntry.IsDir() {
		// dir
		if !rm.recursiveFlagValues.Recursive {
			return errors.New("cannot remove a collection, recurse is not set")
		}

		logger.Debug("removing a collection")
		err = rm.filesystem.RemoveDir(targetPath, rm.recursiveFlagValues.Recursive, rm.forceFlagValues.Force)
		if err != nil {
			return errors.Wrapf(err, "failed to remove a collection %q", targetPath)
		}

		return nil
	}

	// file
	logger.Debug("removing a data object")
	err = rm.filesystem.RemoveFile(targetPath, rm.forceFlagValues.Force)
	if err != nil {
		return errors.Wrapf(err, "failed to remove %q", targetPath)
	}

	return nil
}

package subcmd

import (
	"path"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	commons_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/cyverse/gocommands/commons/wildcard"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:     "mv <data-object-or-collection>... <target-data-object-or-collection>",
	Aliases: []string{"imv", "move"},
	Short:   "Move iRODS data-objects or collections to a target collection, or rename data-object/collection",
	Long:    `This command moves iRODS data-objects or collections to the specified target collection, or renames a single data-object or collection.`,
	RunE:    processMvCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddMvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(mvCmd, false)
	flag.SetWildcardSearchFlags(mvCmd)

	rootCmd.AddCommand(mvCmd)
}

func processMvCommand(command *cobra.Command, args []string) error {
	mv, err := NewMvCommand(command, args)
	if err != nil {
		return err
	}

	return mv.Process()
}

type MvCommand struct {
	command                  *cobra.Command
	wildcardSearchFlagValues *flag.WildcardSearchFlagValues

	commonFlagValues *flag.CommonFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string
}

func NewMvCommand(command *cobra.Command, args []string) (*MvCommand, error) {
	mv := &MvCommand{
		command:                  command,
		commonFlagValues:         flag.GetCommonFlagValues(command),
		wildcardSearchFlagValues: flag.GetWildcardSearchFlagValues(),
	}

	// paths
	mv.sourcePaths = args[:len(args)-1]
	mv.targetPath = args[len(args)-1]

	return mv, nil
}

func (mv *MvCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(mv.command)
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
	mv.account = config.GetSessionConfig().ToIRODSAccount()
	mv.filesystem, err = irods.GetIRODSFSClient(mv.account, false, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer mv.filesystem.Release()

	if mv.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(mv.filesystem, mv.commonFlagValues.Timeout)
	}

	// run
	if len(mv.sourcePaths) >= 2 {
		// multi-source, target must be a dir
		err = mv.ensureTargetIsDir(mv.targetPath)
		if err != nil {
			return errors.Wrapf(err, "target path %q is not a directory", mv.targetPath)
		}
	}

	// Expand wildcards
	if mv.wildcardSearchFlagValues.WildcardSearch {
		mv.sourcePaths, err = wildcard.ExpandWildcards(mv.filesystem, mv.account, mv.sourcePaths, true, true)
		if err != nil {
			return errors.Wrapf(err, "failed to expand wildcards")
		}
	}

	for _, sourcePath := range mv.sourcePaths {
		err = mv.moveOne(sourcePath, mv.targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to move (rename) %q to %q", sourcePath, mv.targetPath)
		}
	}

	return nil
}

func (mv *MvCommand) ensureTargetIsDir(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := mv.account.ClientZone
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := mv.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// not exist
			return types.NewNotDirError(targetPath)
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if !targetEntry.IsDir() {
		return types.NewNotDirError(targetPath)
	}

	return nil
}

func (mv *MvCommand) moveOne(sourcePath string, targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := mv.account.ClientZone
	sourcePath = commons_path.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := mv.filesystem.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	targetPath = commons_path.MakeIRODSTargetFilePath(mv.filesystem, sourcePath, targetPath)

	if sourceEntry.IsDir() {
		// dir
		return mv.moveDir(sourceEntry, targetPath)
	}

	// file
	return mv.moveFile(sourceEntry, targetPath)
}

func (mv *MvCommand) moveFile(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	targetEntry, err := mv.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			logger.Debug("renaming a data object")
			err = mv.filesystem.RenameFileToFile(sourceEntry.Path, targetPath)
			if err != nil {
				return errors.Wrapf(err, "failed to rename %q to %q", sourceEntry.Path, targetPath)
			}

			return nil
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		return types.NewNotFileError(targetPath)
	}

	// overwrite
	err = mv.filesystem.RemoveFile(targetPath, true)
	if err != nil {
		return errors.Wrapf(err, "failed to remove %q for overwriting", targetPath)
	}

	logger.Debug("renaming a data object")
	err = mv.filesystem.RenameFileToFile(sourceEntry.Path, targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to rename %q to %q", sourceEntry.Path, targetPath)
	}

	return nil
}

func (mv *MvCommand) moveDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path": sourceEntry.Path,
		"target_path": targetPath,
	})

	targetEntry, err := mv.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directorywith new name
			logger.Debug("renaming a collection")
			err = mv.filesystem.RenameDirToDir(sourceEntry.Path, targetPath)
			if err != nil {
				return errors.Wrapf(err, "failed to rename %q to %q", sourceEntry.Path, targetPath)
			}

			return nil
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exist
	if targetEntry.IsDir() {
		targetDirPath := path.Join(targetPath, sourceEntry.Name)
		logger.Debug("renaming a collection")
		err = mv.filesystem.RenameDirToDir(sourceEntry.Path, targetDirPath)
		if err != nil {
			return errors.Wrapf(err, "failed to rename %q to %q", sourceEntry.Path, targetDirPath)
		}

		return nil
	}

	// file
	return errors.Errorf("failed to rename a collection %q to a file %q", sourceEntry.Path, targetPath)
}

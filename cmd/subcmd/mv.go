package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var mvCmd = &cobra.Command{
	Use:     "mv [data-object1] [data-object2] [collection1] ... [target collection]",
	Aliases: []string{"imv", "move"},
	Short:   "Move iRODS data-objects or collections to target collection, or rename data-object or collection",
	Long:    `This moves iRODS data-objects or collections to the given target collection, or rename a single data-object or collection.`,
	RunE:    processMvCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddMvCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(mvCmd)

	rootCmd.AddCommand(mvCmd)
}

func processMvCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
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
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := args[len(args)-1]
	sourcePaths := args[:len(args)-1]

	// move
	for _, sourcePath := range sourcePaths {
		err = moveOne(filesystem, sourcePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to perform mv %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	return nil
}

func moveOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "moveOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("renaming a data object %s to %s", sourcePath, targetPath)
		err = filesystem.RenameFile(sourcePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to rename %s to %s: %w", sourcePath, targetPath, err)
		}
	} else {
		// dir
		logger.Debugf("renaming a collection %s to %s", sourcePath, targetPath)
		err = filesystem.RenameDir(sourcePath, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to rename %s to %s: %w", sourcePath, targetPath, err)
		}
	}
	return nil
}

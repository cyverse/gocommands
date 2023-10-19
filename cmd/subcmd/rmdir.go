package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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
	flag.SetCommonFlags(rmdirCmd)

	rootCmd.AddCommand(rmdirCmd)
}

func processRmdirCommand(command *cobra.Command, args []string) error {
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

	for _, targetPath := range args {
		err = removeDirOne(filesystem, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to perform rmdir %s: %w", targetPath, err)
		}
	}
	return nil
}

func removeDirOne(filesystem *irodsclient_fs.FileSystem, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "removeDirOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
		// file
		return xerrors.Errorf("%s is not a collection", targetPath)
	} else {
		// dir
		logger.Debugf("removing a collection %s", targetPath)
		err = filesystem.RemoveDir(targetPath, false, false)
		if err != nil {
			return xerrors.Errorf("failed to rmdir %s: %w", targetPath, err)
		}
	}
	return nil
}

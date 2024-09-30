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

var mkdirCmd = &cobra.Command{
	Use:     "mkdir [collection1] [collection2] ...",
	Aliases: []string{"imkdir"},
	Short:   "Make iRODS collections",
	Long:    `This makes iRODS collections.`,
	RunE:    processMkdirCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddMkdirCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(mkdirCmd, false)

	flag.SetParentsFlags(mkdirCmd)

	rootCmd.AddCommand(mkdirCmd)
}

func processMkdirCommand(command *cobra.Command, args []string) error {
	mkDir, err := NewMkDirCommand(command, args)
	if err != nil {
		return err
	}

	return mkDir.Process()
}

type MkDirCommand struct {
	command *cobra.Command

	parentsFlagValues *flag.ParentsFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewMkDirCommand(command *cobra.Command, args []string) (*MkDirCommand, error) {
	mkDir := &MkDirCommand{
		command: command,

		parentsFlagValues: flag.GetParentsFlagValues(),
	}

	// target paths
	mkDir.targetPaths = args

	return mkDir, nil
}

func (mkDir *MkDirCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(mkDir.command)
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
	mkDir.account = commons.GetSessionConfig().ToIRODSAccount()
	mkDir.filesystem, err = commons.GetIRODSFSClientForSingleOperation(mkDir.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer mkDir.filesystem.Release()

	// run
	for _, targetPath := range mkDir.targetPaths {
		err = mkDir.makeOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to make a directory %q: %w", targetPath, err)
		}
	}
	return nil
}

func (mkDir *MkDirCommand) makeOne(targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "MkDirCommand",
		"function": "makeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := mkDir.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	// dir or not exist
	logger.Debugf("making a directory %q", targetPath)
	err := mkDir.filesystem.MakeDir(targetPath, mkDir.parentsFlagValues.MakeParents)
	if err != nil {
		return xerrors.Errorf("failed to create a directory %q: %w", targetPath, err)
	}

	return nil
}

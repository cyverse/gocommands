package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var cdCmd = &cobra.Command{
	Use:     "cd <collection>",
	Aliases: []string{"icd"},
	Short:   "Change the current working iRODS collection",
	Long:    `This command changes the current working iRODS collection.`,
	RunE:    processCdCommand,
	Args:    cobra.MaximumNArgs(1),
}

func AddCdCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(cdCmd, true)

	rootCmd.AddCommand(cdCmd)
}

func processCdCommand(command *cobra.Command, args []string) error {
	cd, err := NewCdCommand(command, args)
	if err != nil {
		return err
	}

	return cd.Process()
}

type CdCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPath string
}

func NewCdCommand(command *cobra.Command, args []string) (*CdCommand, error) {
	cd := &CdCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	// path
	cd.targetPath = ""
	if len(args) == 0 {
		// move to home dir
		cd.targetPath = "~"
	} else if len(args) == 1 {
		cd.targetPath = args[0]
	}

	return cd, nil
}

func (cd *CdCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(cd.command)
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
	cd.account = config.GetSessionConfig().ToIRODSAccount()
	cd.filesystem, err = irods.GetIRODSFSClient(cd.account, false, false)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer cd.filesystem.Release()

	if cd.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(cd.filesystem, cd.commonFlagValues.Timeout)
	}

	// run
	err = cd.changeWorkingDir(cd.targetPath)
	if err != nil {
		return xerrors.Errorf("failed to change working directory to %q: %w", cd.targetPath, err)
	}

	return nil
}

func (cd *CdCommand) changeWorkingDir(collectionPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CdCommand",
		"function": "changeWorkingDir",
	})

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := cd.account.ClientZone
	collectionPath = path.MakeIRODSPath(cwd, home, zone, collectionPath)

	entry, err := cd.filesystem.StatDir(collectionPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// not exist
			return xerrors.Errorf("directory %q does not exist: %w", collectionPath, err)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", collectionPath, err)
		}
	}

	if !entry.IsDir() {
		// not a directory
		return xerrors.Errorf("%q is not a directory", collectionPath)
	}

	logger.Debugf("changing working directory to %q", collectionPath)

	err = config.SetCWD(collectionPath)
	if err != nil {
		return xerrors.Errorf("failed to set current working collection to %q: %w", collectionPath, err)
	}

	return nil
}

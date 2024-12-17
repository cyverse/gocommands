package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var touchCmd = &cobra.Command{
	Use:     "touch [data-object]",
	Aliases: []string{"itouch"},
	Short:   "Create an empty iRODS data-object or update timestamp of existing data-object",
	Long:    `Create an empty iRODS data-object or update timestamp of existing data-object.`,
	RunE:    processTouchCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddTouchCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(touchCmd, false)

	flag.SetNoCreateFlags(touchCmd)

	rootCmd.AddCommand(touchCmd)
}

func processTouchCommand(command *cobra.Command, args []string) error {
	touch, err := NewTouchCommand(command, args)
	if err != nil {
		return err
	}

	return touch.Process()
}

type TouchCommand struct {
	command *cobra.Command

	commonFlagValues   *flag.CommonFlagValues
	noCreateFlagValues *flag.NoCreateFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewTouchCommand(command *cobra.Command, args []string) (*TouchCommand, error) {
	touch := &TouchCommand{
		command: command,

		commonFlagValues:   flag.GetCommonFlagValues(command),
		noCreateFlagValues: flag.GetNoCreateFlagValues(),
	}

	// path
	touch.targetPaths = args

	return touch, nil
}

func (touch *TouchCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(touch.command)
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
	touch.account = commons.GetSessionConfig().ToIRODSAccount()
	touch.filesystem, err = commons.GetIRODSFSClientForSingleOperation(touch.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer touch.filesystem.Release()

	// run
	for _, targetPath := range touch.targetPaths {
		err = touch.touchOne(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to touch %q: %w", targetPath, err)
		}
	}

	return nil
}

func (touch *TouchCommand) touchOne(targetPath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := touch.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := touch.filesystem.Touch(targetPath, "", touch.noCreateFlagValues.NoCreate)
	if err != nil {
		return xerrors.Errorf("failed to touch file %q: %w", targetPath, err)
	}

	return nil
}

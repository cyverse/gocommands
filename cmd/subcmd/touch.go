package subcmd

import (
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/spf13/cobra"
)

var touchCmd = &cobra.Command{
	Use:     "touch <data-object>",
	Aliases: []string{"itouch"},
	Short:   "Create an empty iRODS data-object or update its timestamp",
	Long:    `This command creates an empty iRODS data-object or updates the timestamp of an existing data-object.`,
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

	commonFlagValues *flag.CommonFlagValues
	touchFlagValues  *flag.TouchFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewTouchCommand(command *cobra.Command, args []string) (*TouchCommand, error) {
	touch := &TouchCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
		touchFlagValues:  flag.GetTouchFlagValues(command),
	}

	// path
	touch.targetPaths = args

	return touch, nil
}

func (touch *TouchCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(touch.command)
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
	touch.account = config.GetSessionConfig().ToIRODSAccount()
	touch.filesystem, err = irods.GetIRODSFSClient(touch.account, true)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer touch.filesystem.Release()

	if touch.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(touch.filesystem, touch.commonFlagValues.Timeout)
	}

	// run
	for _, targetPath := range touch.targetPaths {
		err = touch.touchOne(targetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to touch %q", targetPath)
		}
	}

	return nil
}

func (touch *TouchCommand) touchOne(targetPath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := touch.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	var replicaNumber *int = nil
	if touch.touchFlagValues.ReplicaNumberUpdated {
		replicaNumber = &touch.touchFlagValues.ReplicaNumber
	}

	var seconds *int = nil
	if touch.touchFlagValues.SecondsSinceEpochUpdated {
		seconds = &touch.touchFlagValues.SecondsSinceEpoch
	}

	err := touch.filesystem.Touch(targetPath, touch.commonFlagValues.Resource, touch.touchFlagValues.NoCreate, replicaNumber, touch.touchFlagValues.ReferencePath, seconds)
	if err != nil {
		return errors.Wrapf(err, "failed to touch file %q", targetPath)
	}

	return nil
}

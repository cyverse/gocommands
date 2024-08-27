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

var touchCmd = &cobra.Command{
	Use:     "touch [data-object]",
	Aliases: []string{"itouch"},
	Short:   "Create an empty iRODS data-object or update last modified time of existing data-object",
	Long:    `This displays the content of an iRODS data-object.`,
	RunE:    processTouchCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddTouchCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(touchCmd, false)

	flag.SetTicketAccessFlags(touchCmd)
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

	ticketAccessFlagValues *flag.TicketAccessFlagValues
	noCreateFlagValues     *flag.NoCreateFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewTouchCommand(command *cobra.Command, args []string) (*TouchCommand, error) {
	touch := &TouchCommand{
		command: command,

		ticketAccessFlagValues: flag.GetTicketAccessFlagValues(),
		noCreateFlagValues:     flag.GetNoCreateFlagValues(),
	}

	// path
	touch.targetPaths = args

	return touch, nil
}

func (touch *TouchCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "TouchCommand",
		"function": "Process",
	})

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

	// config
	appConfig := commons.GetConfig()
	syncAccount := false
	if len(touch.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", touch.ticketAccessFlagValues.Name)
		appConfig.Ticket = touch.ticketAccessFlagValues.Name
		syncAccount = true
	}

	if syncAccount {
		err := commons.SyncAccount()
		if err != nil {
			return err
		}
	}

	// Create a file system
	touch.account = commons.GetAccount()
	touch.filesystem, err = commons.GetIRODSFSClient(touch.account)
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
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := touch.filesystem.Touch(targetPath, "", touch.noCreateFlagValues.NoCreate)
	if err != nil {
		return xerrors.Errorf("failed to touch file %q: %w", targetPath, err)
	}

	return nil
}

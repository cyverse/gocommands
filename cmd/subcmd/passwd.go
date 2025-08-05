package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/terminal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var passwdCmd = &cobra.Command{
	Use:     "passwd",
	Aliases: []string{"ipasswd"},
	Short:   "Change iRODS user password",
	Long:    `This command changes the iRODS user password.`,
	RunE:    processPasswdCommand,
	Args:    cobra.NoArgs,
}

func AddPasswdCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(passwdCmd, true)

	rootCmd.AddCommand(passwdCmd)
}

func processPasswdCommand(command *cobra.Command, args []string) error {
	passwd, err := NewPasswdCommand(command, args)
	if err != nil {
		return err
	}

	return passwd.Process()
}

type PasswdCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewPasswdCommand(command *cobra.Command, args []string) (*PasswdCommand, error) {
	passwd := &PasswdCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
	}

	return passwd, nil
}

func (passwd *PasswdCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(passwd.command)
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
	passwd.account = config.GetSessionConfig().ToIRODSAccount()
	passwd.filesystem, err = irods.GetIRODSFSClient(passwd.account, true, false)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer passwd.filesystem.Release()

	if passwd.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(passwd.filesystem, passwd.commonFlagValues.Timeout)
	}

	err = passwd.changePassword()
	if err != nil {
		return xerrors.Errorf("failed to change password: %w", err)
	}
	return nil
}

func (passwd *PasswdCommand) changePassword() error {
	logger := log.WithFields(log.Fields{
		"user": passwd.account.ClientUser,
	})

	logger.Debug("changing password")

	pass := false
	for i := 0; i < 3; i++ {
		currentPassword := terminal.InputPassword("Current iRODS Password")
		if currentPassword == passwd.account.Password {
			pass = true
			break
		}

		terminal.Println("Wrong password")
		terminal.Println("")
	}

	if !pass {
		return xerrors.Errorf("password mismatched")
	}

	pass = false
	newPassword := ""
	for i := 0; i < 3; i++ {
		newPassword = terminal.InputPassword("New iRODS Password")
		if newPassword != passwd.account.Password {
			pass = true
			break
		}

		terminal.Println("Please provide new password")
		terminal.Println("")
	}

	if !pass {
		return xerrors.Errorf("invalid password provided")
	}

	newPasswordConfirm := terminal.InputPassword("Confirm New iRODS Password")
	if newPassword != newPasswordConfirm {
		return xerrors.Errorf("password mismatched")
	}

	err := passwd.filesystem.ChangeUserPassword(passwd.account.ClientUser, passwd.account.ClientZone, newPassword)
	if err != nil {
		return xerrors.Errorf("failed to change user password for user %q: %w", passwd.account.ClientUser, err)
	}

	return nil
}

package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var passwdCmd = &cobra.Command{
	Use:     "passwd",
	Aliases: []string{"ipasswd"},
	Short:   "Change iRODS user password",
	Long:    `This changes iRODS user password.`,
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

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewPasswdCommand(command *cobra.Command, args []string) (*PasswdCommand, error) {
	passwd := &PasswdCommand{
		command: command,
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
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a connection
	passwd.account = commons.GetSessionConfig().ToIRODSAccount()
	passwd.filesystem, err = commons.GetIRODSFSClientForSingleOperation(passwd.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	err = passwd.changePassword()
	if err != nil {
		return xerrors.Errorf("failed to change password: %w", err)
	}
	return nil
}

func (passwd *PasswdCommand) changePassword() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "PasswdCommand",
		"function": "changePassword",
	})

	connection, err := passwd.filesystem.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer passwd.filesystem.ReturnMetadataConnection(connection)

	logger.Debugf("changing password for user %q", passwd.account.ClientUser)

	pass := false
	for i := 0; i < 3; i++ {
		currentPassword := commons.InputPassword("Current iRODS Password")
		if currentPassword == passwd.account.Password {
			pass = true
			break
		}

		commons.Println("Wrong password")
		commons.Println("")
	}

	if !pass {
		return xerrors.Errorf("password mismatched")
	}

	pass = false
	newPassword := ""
	for i := 0; i < 3; i++ {
		newPassword = commons.InputPassword("New iRODS Password")
		if newPassword != passwd.account.Password {
			pass = true
			break
		}

		commons.Println("Please provide new password")
		commons.Println("")
	}

	if !pass {
		return xerrors.Errorf("invalid password provided")
	}

	newPasswordConfirm := commons.InputPassword("Confirm New iRODS Password")
	if newPassword != newPasswordConfirm {
		return xerrors.Errorf("password mismatched")
	}

	err = irodsclient_irodsfs.ChangeUserPassword(connection, passwd.account.ClientUser, passwd.account.ClientZone, newPassword)
	if err != nil {
		return xerrors.Errorf("failed to change user password for user %q: %w", passwd.account.ClientUser, err)
	}

	return nil
}

package subcmd

import (
	"fmt"
	"syscall"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	passwd.account = commons.GetAccount()
	passwd.filesystem, err = commons.GetIRODSFSClient(passwd.account)
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
		fmt.Print("Current iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return xerrors.Errorf("failed to read password: %w", err)
		}

		fmt.Print("\n")
		currentPassword := string(bytePassword)

		if currentPassword == passwd.account.Password {
			pass = true
			break
		}

		fmt.Println("Wrong password")
		fmt.Println("")
	}

	if !pass {
		return xerrors.Errorf("password mismatched")
	}

	pass = false
	newPassword := ""
	for i := 0; i < 3; i++ {
		fmt.Print("New iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return xerrors.Errorf("failed to read password: %w", err)
		}

		fmt.Print("\n")
		newPassword = string(bytePassword)

		if newPassword != passwd.account.Password {
			pass = true
			break
		}

		fmt.Println("Please provide new password")
		fmt.Println("")
	}

	if !pass {
		return xerrors.Errorf("invalid password provided")
	}

	newPasswordConfirm := ""
	fmt.Print("Confirm New iRODS Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return xerrors.Errorf("failed to read password: %w", err)
	}

	fmt.Print("\n")
	newPasswordConfirm = string(bytePassword)

	if newPassword != newPasswordConfirm {
		return xerrors.Errorf("password mismatched")
	}

	err = irodsclient_irodsfs.ChangeUserPassword(connection, passwd.account.ClientUser, passwd.account.ClientZone, newPassword)
	if err != nil {
		return xerrors.Errorf("failed to change user password for user %q: %w", passwd.account.ClientUser, err)
	}

	return nil
}

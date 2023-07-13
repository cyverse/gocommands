package subcmd

import (
	"fmt"
	"syscall"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
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
	commons.SetCommonFlags(passwdCmd)

	rootCmd.AddCommand(passwdCmd)
}

func processPasswdCommand(command *cobra.Command, args []string) error {
	cont, err := commons.ProcessCommonFlags(command)
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
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	err = changePassword(filesystem)
	if err != nil {
		return xerrors.Errorf("failed to change password: %w", err)
	}
	return nil
}

func changePassword(fs *irodsclient_fs.FileSystem) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "changePassword",
	})

	account := commons.GetAccount()

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	logger.Debugf("changing password for user %s", account.ClientUser)

	pass := false
	for i := 0; i < 3; i++ {
		fmt.Print("Current iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return xerrors.Errorf("failed to read password: %w", err)
		}

		fmt.Print("\n")
		currentPassword := string(bytePassword)

		if currentPassword == account.Password {
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

		if newPassword != account.Password {
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

	err = irodsclient_irodsfs.ChangeUserPassword(connection, account.ClientUser, account.ClientZone, newPassword)
	if err != nil {
		return xerrors.Errorf("failed to change user password for user %s: %w", account.ClientUser, err)
	}
	return nil

}

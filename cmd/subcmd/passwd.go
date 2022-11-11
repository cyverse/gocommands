package subcmd

import (
	"fmt"
	"os"
	"syscall"

	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_fs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var passwdCmd = &cobra.Command{
	Use:   "passwd",
	Short: "Change iRODS user password",
	Long:  `This changes iRODS user password.`,
	RunE:  processPasswdCommand,
}

func AddPasswdCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(passwdCmd)

	rootCmd.AddCommand(passwdCmd)
}

func processPasswdCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processPasswdCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	// Create a connection
	account := commons.GetAccount()
	irodsConn, err := commons.GetIRODSConnection(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer irodsConn.Disconnect()

	err = changePassword(irodsConn)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}
	return nil
}

func changePassword(connection *irodsclient_conn.IRODSConnection) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "changePassword",
	})

	account := commons.GetAccount()

	logger.Debugf("changing password for user %s", account.ClientUser)

	pass := false
	for i := 0; i < 3; i++ {
		fmt.Print("Current iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		fmt.Print("\n")
		currentPassword := string(bytePassword)

		if len(currentPassword) == 0 {
			fmt.Println("Please provide password")
			fmt.Println("")
			continue
		}

		if currentPassword == account.Password {
			pass = true
			break
		}

		fmt.Println("Wrong password")
		fmt.Println("")
	}

	if !pass {
		return fmt.Errorf("password mismatched")
	}

	pass = false
	newPassword := ""
	for i := 0; i < 3; i++ {
		fmt.Print("New iRODS Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		fmt.Print("\n")
		newPassword = string(bytePassword)

		if len(newPassword) == 0 {
			fmt.Println("Please provide password")
			fmt.Println("")
			continue
		}

		if newPassword != account.Password {
			pass = true
			break
		}

		fmt.Println("Please provide new password")
		fmt.Println("")
	}

	if !pass {
		return fmt.Errorf("invalid password provided")
	}

	newPasswordConfirm := ""
	fmt.Print("Confirm New iRODS Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}

	fmt.Print("\n")
	newPasswordConfirm = string(bytePassword)

	if newPassword != newPasswordConfirm {
		return fmt.Errorf("password mismatched")
	}

	err = irodsclient_fs.ChangeUserPassword(connection, account.ClientUser, account.ClientZone, newPassword)
	if err != nil {
		return err
	}
	return nil

}

package subcmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var copySftpIdCmd = &cobra.Command{
	Use:   "copy-sftp-id",
	Short: "Copy SSH public key to iRODS for SFTP access",
	Long:  `This copies SSH public key to iRODS for SFTP access.`,
	RunE:  processCopySftpIdCommand,
}

func AddCopySftpIdCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(copySftpIdCmd)
	copySftpIdCmd.Flags().BoolP("force", "f", false, "Copy keys forcefully without duplication check")
	copySftpIdCmd.Flags().BoolP("dryrun", "n", false, "No keys are actually copied")
	copySftpIdCmd.Flags().StringP("identity_file", "i", "", "Specify identity file")

	rootCmd.AddCommand(copySftpIdCmd)
}

func processCopySftpIdCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCopySftpIdCommand",
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	dryrun := false
	dryrunFlag := command.Flags().Lookup("dryrun")
	if dryrunFlag != nil {
		dryrun, err = strconv.ParseBool(dryrunFlag.Value.String())
		if err != nil {
			dryrun = false
		}
	}

	identityFile := ""
	identityFileFlag := command.Flags().Lookup("identity_file")
	if identityFileFlag != nil {
		identityFile = identityFileFlag.Value.String()
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	// search identity files to be copied
	identityFiles := []string{}
	if len(identityFile) > 0 {
		// if identity file is given via flag
		identityFilePath := commons.MakeLocalPath(identityFile)
		identityFiles = append(identityFiles, identityFilePath)
	} else {
		// scan defaults
		identityFiles, err = scanSSHIdentityFiles()
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	if len(identityFiles) == 0 {
		errorMessage := "failed to find SSH identity files '~/.ssh/*.pub'"
		logger.Error(errorMessage)
		fmt.Fprintln(os.Stderr, errorMessage)
		return nil
	}

	err = copySftpId(filesystem, force, dryrun, identityFiles)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}
	return nil
}

func scanSSHIdentityFiles() ([]string, error) {
	// ~/.ssh/*.pub
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshPath := filepath.Join(homePath, ".ssh")

	sshDirEntries, err := os.ReadDir(sshPath)
	if err != nil {
		return nil, err
	}

	identityFiles := []string{}
	for _, sshDirEntry := range sshDirEntries {
		if !sshDirEntry.IsDir() {
			// must be a file
			if strings.HasSuffix(sshDirEntry.Name(), ".pub") {
				// found
				identityFileFullPath := filepath.Join(sshPath, sshDirEntry.Name())
				identityFiles = append(identityFiles, identityFileFullPath)
			}
		}
	}

	return identityFiles, nil
}

func copySftpId(filesystem *irodsclient_fs.FileSystem, force bool, dryrun bool, identityFiles []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "copySftpId",
	})

	account := commons.GetAccount()

	home := commons.GetHomeDir()
	irodsSshPath := path.Join(home, ".ssh")
	authorizedKeyPath := path.Join(irodsSshPath, "authorized_keys")

	if !filesystem.ExistsDir(irodsSshPath) {
		logger.Debugf("SSH directory %s does not exist on iRODS for user %s, creating one", irodsSshPath, account.ClientUser)
		if !dryrun {
			// create ssh dir
			err := filesystem.MakeDir(irodsSshPath, true)
			if err != nil {
				return err
			}
		}
	}

	// read existing authorized_keys
	authorizedKeysArray := []string{}
	if filesystem.ExistsFile(authorizedKeyPath) {
		logger.Debugf("reading authorized_keys %s on iRODS for user %s", authorizedKeyPath, account.ClientUser)

		handle, err := filesystem.OpenFile(authorizedKeyPath, "", "r")
		if err != nil {
			return err
		}
		defer handle.Close()

		sb := strings.Builder{}
		readBuffer := make([]byte, 1024)
		for {
			readLen, err := handle.Read(readBuffer)
			if err != nil && err != io.EOF {
				return err
			}

			_, err2 := sb.Write(readBuffer[:readLen])
			if err2 != nil {
				return err2
			}

			if err == io.EOF {
				break
			}
		}

		existingAuthorizedKeysContent := sb.String()
		authorizedKeysArray = strings.Split(existingAuthorizedKeysContent, "\n")
	}

	contentChanged := false
	// add
	for _, identityFile := range identityFiles {
		logger.Debugf("copying a SSH public key %s to iRODS for user %s", identityFile, account.ClientUser)

		// copy
		// read the identity file first
		identityFileContent, err := ioutil.ReadFile(identityFile)
		if err != nil {
			return err
		}

		userKey, _, _, _, err := ssh.ParseAuthorizedKey(identityFileContent)
		if err != nil {
			log.Debugf("failed to parse a SSH public key %s for user %s", identityFile, account.ClientUser)
			return err
		}

		if force {
			// append forcefully
			authorizedKeysArray = append(authorizedKeysArray, string(identityFileContent))
			contentChanged = true
		} else {
			// check if exists, add only if it doesn't
			hasExisting := false
			for keyLineIdx, keyLine := range authorizedKeysArray {
				keyLine = strings.TrimSpace(keyLine)
				if keyLine == "" || keyLine[0] == '#' {
					// skip
					continue
				}

				authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyLine))
				if err != nil {
					// skip
					log.Debugf("failed to parse a authorized key line - %s", err.Error())
					continue
				}

				if bytes.Equal(authorizedKey.Marshal(), userKey.Marshal()) {
					// existing - update
					authorizedKeysArray[keyLineIdx] = string(identityFileContent)
					hasExisting = true
					contentChanged = true
					break
				}
			}

			if !hasExisting {
				// not found - add
				authorizedKeysArray = append(authorizedKeysArray, string(identityFileContent))
				contentChanged = true
			}
		}
	}

	// upload
	if !dryrun {
		if !contentChanged {
			logger.Debugf("skipping writing authorized_keys %s on iRODS for user %s, nothing changed", authorizedKeyPath, account.ClientUser)
		} else {
			logger.Debugf("writing authorized_keys %s on iRODS for user %s", authorizedKeyPath, account.ClientUser)

			// open the file with write truncate mode
			handle, err := filesystem.OpenFile(authorizedKeyPath, "", "w+")
			if err != nil {
				return err
			}
			defer handle.Close()

			buf := bytes.Buffer{}
			for _, key := range authorizedKeysArray {
				key = strings.TrimSpace(key)
				if len(key) > 0 {
					buf.WriteString(key)
					buf.WriteString("\n")
				}
			}

			_, err = handle.Write(buf.Bytes())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

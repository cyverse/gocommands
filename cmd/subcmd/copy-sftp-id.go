package subcmd

import (
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var copySftpIdCmd = &cobra.Command{
	Use:     "copy-sftp-id",
	Aliases: []string{"copy_sftp_id"},
	Short:   "Copy SSH public key to iRODS for SFTP access",
	Long:    `This copies SSH public key to iRODS for SFTP access.`,
	RunE:    processCopySftpIdCommand,
	Args:    cobra.NoArgs,
}

func AddCopySftpIdCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(copySftpIdCmd)

	flag.SetForceFlags(copySftpIdCmd, false)
	flag.SetDryRunFlags(copySftpIdCmd)
	flag.SetSFTPIDFlags(copySftpIdCmd)

	rootCmd.AddCommand(copySftpIdCmd)
}

func processCopySftpIdCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
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

	forceFlagValues := flag.GetForceFlagValues()
	dryRunFlagValues := flag.GetDryRunFlagValues()
	sftpIDFlagValues := flag.GetSFTPIDFlagValues()

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	// search identity files to be copied
	identityFiles := []string{}
	if len(sftpIDFlagValues.IdentityFilePath) > 0 {
		// if identity file is given via flag
		identityFilePath := commons.MakeLocalPath(sftpIDFlagValues.IdentityFilePath)
		identityFiles = append(identityFiles, identityFilePath)
	} else {
		// scan defaults
		identityFiles, err = scanSSHIdentityFiles()
		if err != nil {
			return xerrors.Errorf("failed to scan ssh identity files: %w", err)
		}
	}

	if len(identityFiles) == 0 {
		return xerrors.Errorf("failed to find SSH identity files")
	}

	err = copySftpId(filesystem, forceFlagValues.Force, dryRunFlagValues.DryRun, identityFiles)
	if err != nil {
		return xerrors.Errorf("failed to perform copy-sftp-id: %w", err)
	}
	return nil
}

func scanSSHIdentityFiles() ([]string, error) {
	// ~/.ssh/*.pub
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, xerrors.Errorf("failed to get user home dir: %w", err)
	}

	sshPath := filepath.Join(homePath, ".ssh")

	sshDirEntries, err := os.ReadDir(sshPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read dir %s: %w", sshPath, err)
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
				return xerrors.Errorf("failed to make dir %s: %w", irodsSshPath, err)
			}
		}
	}

	// read existing authorized_keys
	authorizedKeysArray := []string{}
	if filesystem.ExistsFile(authorizedKeyPath) {
		logger.Debugf("reading authorized_keys %s on iRODS for user %s", authorizedKeyPath, account.ClientUser)

		handle, err := filesystem.OpenFile(authorizedKeyPath, "", "r")
		if err != nil {
			return xerrors.Errorf("failed to open file %s: %w", authorizedKeyPath, err)
		}
		defer handle.Close()

		sb := strings.Builder{}
		readBuffer := make([]byte, 1024)
		for {
			readLen, err := handle.Read(readBuffer)
			if err != nil && err != io.EOF {
				return xerrors.Errorf("failed to read file %s: %w", authorizedKeyPath, err)
			}

			_, err2 := sb.Write(readBuffer[:readLen])
			if err2 != nil {
				return xerrors.Errorf("failed to write to buffer: %w", err2)
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
		identityFileContent, err := os.ReadFile(identityFile)
		if err != nil {
			return xerrors.Errorf("failed to read file %s: %w", identityFile, err)
		}

		userKey, _, _, _, err := ssh.ParseAuthorizedKey(identityFileContent)
		if err != nil {
			return xerrors.Errorf("failed to parse a SSH public key %s for user %s: %w", identityFile, account.ClientUser, err)
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
				return xerrors.Errorf("failed to open file %s: %w", authorizedKeyPath, err)
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
				return xerrors.Errorf("failed to write: %w", err)
			}
		}
	}

	return nil
}

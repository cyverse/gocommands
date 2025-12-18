package subcmd

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	common_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var copySftpIdCmd = &cobra.Command{
	Use:     "copy-sftp-id",
	Aliases: []string{"copy_sftp_id"},
	Short:   "Copy SSH public key to iRODS for SFTP access",
	Long:    `This command copies an SSH public key to iRODS to enable public-key authentication for SFTP access.`,
	RunE:    processCopySftpIdCommand,
	Args:    cobra.NoArgs,
}

func AddCopySftpIdCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(copySftpIdCmd, false)

	flag.SetDryRunFlags(copySftpIdCmd)
	flag.SetSFTPIDFlags(copySftpIdCmd)

	rootCmd.AddCommand(copySftpIdCmd)
}

func processCopySftpIdCommand(command *cobra.Command, args []string) error {
	copy, err := NewCopySftpIdCommand(command, args)
	if err != nil {
		return err
	}

	return copy.Process()
}

type CopySftpIdCommand struct {
	command *cobra.Command

	commonFlagValues *flag.CommonFlagValues
	dryRunFlagValues *flag.DryRunFlagValues
	sftpIDFlagValues *flag.SFTPIDFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewCopySftpIdCommand(command *cobra.Command, args []string) (*CopySftpIdCommand, error) {
	copy := &CopySftpIdCommand{
		command: command,

		commonFlagValues: flag.GetCommonFlagValues(command),
		dryRunFlagValues: flag.GetDryRunFlagValues(),
		sftpIDFlagValues: flag.GetSFTPIDFlagValues(),
	}

	return copy, nil
}

func (copy *CopySftpIdCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(copy.command)
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
	copy.account = config.GetSessionConfig().ToIRODSAccount()
	copy.filesystem, err = irods.GetIRODSFSClient(copy.account, false, false)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer copy.filesystem.Release()

	if copy.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(copy.filesystem, copy.commonFlagValues.Timeout)
	}

	// run
	// search identity files to be copied
	identityFiles, err := copy.scanSSHIdentityFiles()
	if err != nil {
		return errors.Wrapf(err, "failed to find SSH identity files")
	}

	err = copy.copySftpId(identityFiles)
	if err != nil {
		return errors.Wrapf(err, "failed to copy sftp public key")
	}

	return nil
}

func (copy *CopySftpIdCommand) scanSSHIdentityFiles() ([]string, error) {
	if len(copy.sftpIDFlagValues.IdentityFilePath) > 0 {
		// if identity file is given via flag
		identityFilePath := common_path.MakeLocalPath(copy.sftpIDFlagValues.IdentityFilePath)
		_, err := os.Stat(identityFilePath)
		if err != nil {
			return nil, err
		}

		return []string{identityFilePath}, nil
	}

	// scan defaults
	identityFiles, err := copy.scanDefaultSSHIdentityFiles()
	if err != nil {
		return nil, err
	}

	if len(identityFiles) == 0 {
		return nil, errors.Errorf("failed to find SSH identity files")
	}

	return identityFiles, nil
}

func (copy *CopySftpIdCommand) scanDefaultSSHIdentityFiles() ([]string, error) {
	// ~/.ssh/*.pub
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user home directory")
	}

	sshPath := filepath.Join(homePath, ".ssh")

	sshDirEntries, err := os.ReadDir(sshPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read a directory %q", sshPath)
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

func (copy *CopySftpIdCommand) readAuthorizedKeys(authorizedKeyPath string) ([]string, error) {
	logger := log.WithFields(log.Fields{
		"authorized_key_path": authorizedKeyPath,
		"user":                copy.account.ClientUser,
	})

	if copy.filesystem.ExistsFile(authorizedKeyPath) {
		logger.Debug("reading authorized_keys")

		contentBuffer := bytes.Buffer{}

		_, err := copy.filesystem.DownloadFileToBuffer(authorizedKeyPath, "", &contentBuffer, true, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %q", authorizedKeyPath)
		}

		existingAuthorizedKeysContent := contentBuffer.String()
		if len(existingAuthorizedKeysContent) > 0 {
			existingAuthorizedKeysContent = strings.TrimSpace(existingAuthorizedKeysContent)
			authorizedKeysArray := strings.Split(existingAuthorizedKeysContent, "\n")
			return authorizedKeysArray, nil
		}
	}

	return []string{}, nil
}

func (copy *CopySftpIdCommand) updateAuthorizedKeys(identityFiles []string, authorizedKeys []string) ([]string, bool, error) {
	contentChanged := false
	newAuthorizedKeys := []string{}

	newAuthorizedKeys = append(newAuthorizedKeys, authorizedKeys...)

	// add
	for _, identityFile := range identityFiles {
		logger := log.WithFields(log.Fields{
			"identity_file":   identityFile,
			"authorized_keys": authorizedKeys,
			"user":            copy.account.ClientUser,
		})

		logger.Debug("checking a SSH public key")

		// copy
		// read the identity file first
		identityFileContent, err := os.ReadFile(identityFile)
		if err != nil {
			return newAuthorizedKeys, contentChanged, errors.Wrapf(err, "failed to read file %q", identityFile)
		}

		userKey, _, _, _, err := ssh.ParseAuthorizedKey(identityFileContent)
		if err != nil {
			return newAuthorizedKeys, contentChanged, errors.Wrapf(err, "failed to parse a SSH public key %q for user %q", identityFile, copy.account.ClientUser)
		}

		// check if exists, add only if it doesn't
		hasExisting := false
		for keyLineIdx, keyLine := range newAuthorizedKeys {
			keyLine = strings.TrimSpace(keyLine)
			if keyLine == "" || keyLine[0] == '#' {
				// skip
				continue
			}

			authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyLine))
			if err != nil {
				// skip
				log.WithError(err).Debugf("failed to parse a authorized key line")
				continue
			}

			if bytes.Equal(authorizedKey.Marshal(), userKey.Marshal()) {
				// existing - update
				newAuthorizedKeys[keyLineIdx] = string(identityFileContent)
				hasExisting = true
				break
			}
		}

		if !hasExisting {
			// not found - add
			newAuthorizedKeys = append(newAuthorizedKeys, string(identityFileContent))
			contentChanged = true
			logger.Debug("adding a SSH public key")
		}
	}

	return newAuthorizedKeys, contentChanged, nil
}

func (copy *CopySftpIdCommand) copySftpId(identityFiles []string) error {
	home := config.GetHomeDir()
	irodsSshPath := path.Join(home, ".ssh")
	authorizedKeyPath := path.Join(irodsSshPath, "authorized_keys")

	logger := log.WithFields(log.Fields{
		"identity_files": identityFiles,
		"user":           copy.account.ClientUser,
		"ssh_path":       irodsSshPath,
		"authorized_key": authorizedKeyPath,
	})

	if !copy.filesystem.ExistsDir(irodsSshPath) {
		logger.Debugf("SSH collection does not exist, creating one")

		if !copy.dryRunFlagValues.DryRun {
			// create ssh dir
			err := copy.filesystem.MakeDir(irodsSshPath, true)
			if err != nil {
				return errors.Wrapf(err, "failed to make a collection %q", irodsSshPath)
			}
		}
	}

	// read existing authorized_keys
	authorizedKeys, err := copy.readAuthorizedKeys(authorizedKeyPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read authorized_keys %q", authorizedKeyPath)
	}

	authorizedKeysUpdated, contentChanged, err := copy.updateAuthorizedKeys(identityFiles, authorizedKeys)
	if err != nil {
		return errors.Wrapf(err, "failed to update authorized_keys %q", authorizedKeyPath)
	}

	contentBuf := bytes.Buffer{}
	for _, key := range authorizedKeysUpdated {
		key = strings.TrimSpace(key)
		if len(key) > 0 {
			contentBuf.WriteString(key + "\n")
		}
	}

	// upload
	if !copy.dryRunFlagValues.DryRun {
		if !contentChanged {
			logger.Debug("skipping writing authorized_keys, nothing changed")
			terminal.Printf("SSH public key(s) are already set for user %q\n", copy.account.ClientUser)
			return nil
		}

		logger.Debug("writing authorized_keys")

		_, err := copy.filesystem.UploadFileFromBuffer(&contentBuf, authorizedKeyPath, "", false, true, false, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to update keys in %q", authorizedKeyPath)
		}

		terminal.Printf("%d SSH public key(s) copied to iRODS for user %q\n", len(authorizedKeysUpdated), copy.account.ClientUser)
	}

	return nil
}

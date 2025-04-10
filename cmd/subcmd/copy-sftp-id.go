package subcmd

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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

	// Create a file system
	copy.account = commons.GetSessionConfig().ToIRODSAccount()
	copy.filesystem, err = commons.GetIRODSFSClient(copy.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer copy.filesystem.Release()

	// run
	// search identity files to be copied
	identityFiles, err := copy.scanSSHIdentityFiles()
	if err != nil {
		return xerrors.Errorf("failed to find SSH identity files: %w", err)
	}

	err = copy.copySftpId(identityFiles)
	if err != nil {
		return xerrors.Errorf("failed to copy sftp public key: %w", err)
	}

	return nil
}

func (copy *CopySftpIdCommand) scanSSHIdentityFiles() ([]string, error) {
	if len(copy.sftpIDFlagValues.IdentityFilePath) > 0 {
		// if identity file is given via flag
		identityFilePath := commons.MakeLocalPath(copy.sftpIDFlagValues.IdentityFilePath)
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
		return nil, xerrors.Errorf("failed to find SSH identity files")
	}

	return identityFiles, nil
}

func (copy *CopySftpIdCommand) scanDefaultSSHIdentityFiles() ([]string, error) {
	// ~/.ssh/*.pub
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, xerrors.Errorf("failed to get user home directory: %w", err)
	}

	sshPath := filepath.Join(homePath, ".ssh")

	sshDirEntries, err := os.ReadDir(sshPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read a directory %q: %w", sshPath, err)
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
		"package":  "subcmd",
		"struct":   "CopySftpIdCommand",
		"function": "readAuthorizedKeys",
	})

	if copy.filesystem.ExistsFile(authorizedKeyPath) {
		logger.Debugf("reading authorized_keys %q on iRODS for user %q", authorizedKeyPath, copy.account.ClientUser)

		contentBuffer := bytes.Buffer{}

		_, err := copy.filesystem.DownloadFileToBuffer(authorizedKeyPath, "", &contentBuffer, true, nil)
		if err != nil {
			return nil, xerrors.Errorf("failed to read file %q: %w", authorizedKeyPath, err)
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
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CopySftpIdCommand",
		"function": "updateAuthorizedKeys",
	})

	contentChanged := false
	newAuthorizedKeys := []string{}

	newAuthorizedKeys = append(newAuthorizedKeys, authorizedKeys...)

	// add
	for _, identityFile := range identityFiles {
		logger.Debugf("checking a SSH public key %q to iRODS for user %q", identityFile, copy.account.ClientUser)

		// copy
		// read the identity file first
		identityFileContent, err := os.ReadFile(identityFile)
		if err != nil {
			return newAuthorizedKeys, contentChanged, xerrors.Errorf("failed to read file %q: %w", identityFile, err)
		}

		userKey, _, _, _, err := ssh.ParseAuthorizedKey(identityFileContent)
		if err != nil {
			return newAuthorizedKeys, contentChanged, xerrors.Errorf("failed to parse a SSH public key %q for user %q: %w", identityFile, copy.account.ClientUser, err)
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
			logger.Debugf("adding a SSH public key %q to iRODS for user %q", identityFile, copy.account.ClientUser)
		}
	}

	return newAuthorizedKeys, contentChanged, nil
}

func (copy *CopySftpIdCommand) copySftpId(identityFiles []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CopySftpIdCommand",
		"function": "copySftpId",
	})

	home := commons.GetHomeDir()
	irodsSshPath := path.Join(home, ".ssh")
	authorizedKeyPath := path.Join(irodsSshPath, "authorized_keys")

	if !copy.filesystem.ExistsDir(irodsSshPath) {
		logger.Debugf("SSH directory %q does not exist on iRODS for user %q, creating one", irodsSshPath, copy.account.ClientUser)

		if !copy.dryRunFlagValues.DryRun {
			// create ssh dir
			err := copy.filesystem.MakeDir(irodsSshPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make a directory %q: %w", irodsSshPath, err)
			}
		}
	}

	// read existing authorized_keys
	authorizedKeys, err := copy.readAuthorizedKeys(authorizedKeyPath)
	if err != nil {
		return xerrors.Errorf("failed to read authorized_keys %q: %w", authorizedKeyPath, err)
	}

	authorizedKeysUpdated, contentChanged, err := copy.updateAuthorizedKeys(identityFiles, authorizedKeys)
	if err != nil {
		return xerrors.Errorf("failed to update authorized_keys: %w", err)
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
			logger.Debugf("skipping writing authorized_keys %q on iRODS for user %q, nothing changed", authorizedKeyPath, copy.account.ClientUser)
			commons.Printf("SSH public key(s) are already set for user %q\n", copy.account.ClientUser)
			return nil
		}

		logger.Debugf("writing authorized_keys %q on iRODS for user %q", authorizedKeyPath, copy.account.ClientUser)

		_, err := copy.filesystem.UploadFileFromBuffer(&contentBuf, authorizedKeyPath, "", false, true, true, false, nil)
		if err != nil {
			return xerrors.Errorf("failed to update keys in %q: %w", authorizedKeyPath, err)
		}

		commons.Printf("%d SSH public key(s) copied to iRODS for user %q\n", len(authorizedKeysUpdated), copy.account.ClientUser)
	}

	return nil
}

package flag

import (
	"os"

	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type EncryptionFlagValues struct {
	Encryption      bool
	Mode            commons.EncryptionMode
	modeInput       string
	EncryptFilename bool
	Key             string
	PrivateKeyPath  string
	TempPath        string
}

type DecryptionFlagValues struct {
	Decryption     bool
	Key            string
	PrivateKeyPath string
	TempPath       string
}

var (
	encryptionFlagValues EncryptionFlagValues
	decryptionFlagValues DecryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	pubkeyPath, _ := commons.ExpandHomeDir("~/.ssh/id_rsa.pub")
	st, _ := os.Stat(pubkeyPath)
	if st == nil {
		// not exist
		// use private key
		pubkeyPath, _ = commons.ExpandHomeDir("~/.ssh/id_rsa")
	}

	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Encrypt files")
	command.Flags().StringVar(&encryptionFlagValues.modeInput, "encrypt_mode", "ssh", "Encryption mode ('winscp', 'pgp', or 'ssh')")
	command.Flags().StringVar(&encryptionFlagValues.Key, "encrypt_key", "", "Encryption key for 'winscp' and 'pgp' mode")
	command.Flags().StringVar(&encryptionFlagValues.PrivateKeyPath, "encrypt_priv_key", pubkeyPath, "Encryption public (or private) key for 'ssh' mode")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Specify temp directory path for encrypting files")

}

func SetDecryptionFlags(command *cobra.Command) {
	privkeyPath, _ := commons.ExpandHomeDir("~/.ssh/id_rsa")

	command.Flags().BoolVar(&decryptionFlagValues.Decryption, "decrypt", false, "Decrypt files")
	command.Flags().StringVar(&decryptionFlagValues.Key, "decrypt_key", "", "Decryption key for 'winscp' and 'pgp' mode")
	command.Flags().StringVar(&encryptionFlagValues.PrivateKeyPath, "decrypt_priv_key", privkeyPath, "Decryption private key for 'ssh' mode")
	command.Flags().StringVar(&decryptionFlagValues.TempPath, "decrypt_temp", os.TempDir(), "Specify temp directory path for decrypting files")
}

func GetEncryptionFlagValues() *EncryptionFlagValues {
	encryptionFlagValues.Mode = commons.GetEncryptionMode(encryptionFlagValues.modeInput)
	if encryptionFlagValues.EncryptFilename {
		encryptionFlagValues.Encryption = true
	}

	if len(encryptionFlagValues.Key) > 0 {
		encryptionFlagValues.Encryption = true
	}

	return &encryptionFlagValues
}

func GetDecryptionFlagValues() *DecryptionFlagValues {
	if len(decryptionFlagValues.Key) > 0 {
		decryptionFlagValues.Decryption = true
	}

	return &decryptionFlagValues
}

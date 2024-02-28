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
	TempPath        string
}

type DecryptionFlagValues struct {
	Decryption bool
	Key        string
	TempPath   string
}

var (
	encryptionFlagValues EncryptionFlagValues
	decryptionFlagValues DecryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Encrypt files")
	command.Flags().StringVar(&encryptionFlagValues.modeInput, "encrypt_mode", "pgp", "Encryption mode (pgp only for now)")
	command.Flags().BoolVar(&encryptionFlagValues.EncryptFilename, "encrypt_filename", false, "Encrypt filename (only for pgp)")
	command.Flags().StringVar(&encryptionFlagValues.Key, "encrypt_key", "", "Encryption key")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Specify temp directory path for encrypting files")
}

func SetDecryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&decryptionFlagValues.Decryption, "decrypt", false, "Decrypt files")
	command.Flags().StringVar(&decryptionFlagValues.Key, "decrypt_key", "", "Decryption key")
	command.Flags().StringVar(&decryptionFlagValues.TempPath, "decrypt_temp", os.TempDir(), "Specify temp directory path for decrypting files")
}

func GetEncryptionFlagValues() *EncryptionFlagValues {
	encryptionFlagValues.Mode = commons.GetEncryptionMode(encryptionFlagValues.modeInput)
	if encryptionFlagValues.Mode == commons.EncryptionModeWinSCP {
		encryptionFlagValues.EncryptFilename = true
	}

	return &encryptionFlagValues
}

func GetDecryptionFlagValues() *DecryptionFlagValues {
	return &decryptionFlagValues
}

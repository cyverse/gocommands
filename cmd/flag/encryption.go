package flag

import (
	"os"

	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type EncryptionFlagValues struct {
	Encryption           bool
	NoEncryption         bool
	IgnoreMeta           bool
	Mode                 commons.EncryptionMode
	modeInput            string
	Key                  string
	PublicPrivateKeyPath string
	TempPath             string
}

type DecryptionFlagValues struct {
	Decryption     bool
	NoDecryption   bool
	IgnoreMeta     bool
	Key            string
	PrivateKeyPath string
	TempPath       string
}

var (
	encryptionFlagValues EncryptionFlagValues
	decryptionFlagValues DecryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Encrypt files")
	command.Flags().BoolVar(&encryptionFlagValues.NoEncryption, "no_encrypt", false, "Disable encryption forcefully")
	command.Flags().BoolVar(&encryptionFlagValues.IgnoreMeta, "ignore_meta", false, "Ignore encryption config via metadata")
	command.Flags().StringVar(&encryptionFlagValues.modeInput, "encrypt_mode", "ssh", "Encryption mode ('winscp', 'pgp', or 'ssh')")
	command.Flags().StringVar(&encryptionFlagValues.Key, "encrypt_key", "", "Encryption key for 'winscp' and 'pgp' mode")
	command.Flags().StringVar(&encryptionFlagValues.PublicPrivateKeyPath, "encrypt_pub_key", commons.GetDefaultPublicKeyPath(), "Encryption public (or private) key for 'ssh' mode")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Specify temp directory path for encrypting files")
}

func SetDecryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&decryptionFlagValues.Decryption, "decrypt", false, "Decrypt files")
	command.Flags().BoolVar(&decryptionFlagValues.NoDecryption, "no_decrypt", false, "Disable decryption forcefully")
	command.Flags().BoolVar(&decryptionFlagValues.IgnoreMeta, "ignore_meta", false, "Ignore decryption config via metadata")
	command.Flags().StringVar(&decryptionFlagValues.Key, "decrypt_key", "", "Decryption key for 'winscp' and 'pgp' mode")
	command.Flags().StringVar(&decryptionFlagValues.PrivateKeyPath, "decrypt_priv_key", commons.GetDefaultPrivateKeyPath(), "Decryption private key for 'ssh' mode")
	command.Flags().StringVar(&decryptionFlagValues.TempPath, "decrypt_temp", os.TempDir(), "Specify temp directory path for decrypting files")
}

func GetEncryptionFlagValues(command *cobra.Command) *EncryptionFlagValues {
	encryptionFlagValues.Mode = commons.GetEncryptionMode(encryptionFlagValues.modeInput)
	if command.Flags().Changed("encrypt_key") && len(encryptionFlagValues.Key) > 0 {
		encryptionFlagValues.Encryption = true
	}

	if encryptionFlagValues.NoEncryption {
		encryptionFlagValues.Encryption = false
	}

	return &encryptionFlagValues
}

func GetDecryptionFlagValues(command *cobra.Command) *DecryptionFlagValues {
	if command.Flags().Changed("decrypt_key") && len(decryptionFlagValues.Key) > 0 {
		decryptionFlagValues.Decryption = true
	}

	if decryptionFlagValues.NoDecryption {
		decryptionFlagValues.Decryption = false
	}

	return &decryptionFlagValues
}

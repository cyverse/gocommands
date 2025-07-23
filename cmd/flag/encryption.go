package flag

import (
	"os"

	"github.com/cyverse/gocommands/commons/encryption"
	"github.com/spf13/cobra"
)

type EncryptionFlagValues struct {
	Encryption           bool
	NoEncryption         bool
	IgnoreMeta           bool
	Mode                 encryption.EncryptionMode
	modeInput            string
	Key                  string
	PublicPrivateKeyPath string
	TempPath             string
}

type DecryptionFlagValues struct {
	Decryption     bool
	NoDecryption   bool
	Key            string
	PrivateKeyPath string
	TempPath       string
}

var (
	encryptionFlagValues EncryptionFlagValues
	decryptionFlagValues DecryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Enable file encryption")
	command.Flags().BoolVar(&encryptionFlagValues.NoEncryption, "no_encrypt", false, "Disable file encryption forcefully")
	command.Flags().BoolVar(&encryptionFlagValues.IgnoreMeta, "ignore_meta", false, "Ignore encryption config via metadata")
	command.Flags().StringVar(&encryptionFlagValues.modeInput, "encrypt_mode", "ssh", "Specify encryption mode ('winscp', 'pgp', or 'ssh')")
	command.Flags().StringVar(&encryptionFlagValues.Key, "encrypt_key", "", "Specify the encryption key for 'winscp' and 'pgp' mode")
	command.Flags().StringVar(&encryptionFlagValues.PublicPrivateKeyPath, "encrypt_pub_key", encryption.GetDefaultPublicKeyPath(), "Provide the encryption public (or private) key for 'ssh' mode")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Set a temporary directory path for file encryption")
}

func SetDecryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&decryptionFlagValues.Decryption, "decrypt", true, "Enable file decryption")
	command.Flags().BoolVar(&decryptionFlagValues.NoDecryption, "no_decrypt", false, "Disable file decryption forcefully")
	command.Flags().StringVar(&decryptionFlagValues.Key, "decrypt_key", "", "Specify the decryption key for 'winscp' or 'pgp' modes")
	command.Flags().StringVar(&decryptionFlagValues.PrivateKeyPath, "decrypt_priv_key", encryption.GetDefaultPrivateKeyPath(), "Provide the decryption private key for 'ssh' mode")
	command.Flags().StringVar(&decryptionFlagValues.TempPath, "decrypt_temp", os.TempDir(), "Set a temporary directory for file decryption")
}

func GetEncryptionFlagValues(command *cobra.Command) *EncryptionFlagValues {
	encryptionFlagValues.Mode = encryption.GetEncryptionMode(encryptionFlagValues.modeInput)
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

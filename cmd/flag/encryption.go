package flag

import (
	"os"

	"github.com/spf13/cobra"
)

type EncryptionFlagValues struct {
	Encryption bool
	Password   string
	TempPath   string
}

var (
	encryptionFlagValues EncryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Encrypt or decrypt files using PGP")
	command.Flags().StringVar(&encryptionFlagValues.Password, "encrypt_password", "", "Encryption password")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Specify temp directory path to create encrypted files")
}

func GetEncryptFlagValues() *EncryptionFlagValues {
	return &encryptionFlagValues
}

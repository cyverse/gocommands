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
	Password        string
	TempPath        string
}

var (
	encryptionFlagValues EncryptionFlagValues
)

func SetEncryptionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&encryptionFlagValues.Encryption, "encrypt", false, "Encrypt or decrypt files")
	command.Flags().StringVar(&encryptionFlagValues.modeInput, "encrypt_mode", "winscp", "Encryption mode (pgp or winscp)")
	command.Flags().BoolVar(&encryptionFlagValues.EncryptFilename, "encrypt_filename", false, "Encryption filename (only for pgp)")
	command.Flags().StringVar(&encryptionFlagValues.Password, "encrypt_password", "", "Encryption password")
	command.Flags().StringVar(&encryptionFlagValues.TempPath, "encrypt_temp", os.TempDir(), "Specify temp directory path to create encrypted files")
}

func GetEncryptFlagValues() *EncryptionFlagValues {
	encryptionFlagValues.Mode = commons.GetEncryptionMode(encryptionFlagValues.modeInput)
	if encryptionFlagValues.Mode == commons.EncryptionModeWinSCP {
		encryptionFlagValues.EncryptFilename = true
	}

	return &encryptionFlagValues
}

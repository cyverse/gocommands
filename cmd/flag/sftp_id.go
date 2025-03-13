package flag

import (
	"github.com/spf13/cobra"
)

type SFTPIDFlagValues struct {
	IdentityFilePath string
}

var (
	sftpIDFlagValues SFTPIDFlagValues
)

func SetSFTPIDFlags(command *cobra.Command) {
	command.Flags().StringVarP(&sftpIDFlagValues.IdentityFilePath, "identity_file", "i", "", "Specify the path to the SSH private key file")
}

func GetSFTPIDFlagValues() *SFTPIDFlagValues {
	return &sftpIDFlagValues
}

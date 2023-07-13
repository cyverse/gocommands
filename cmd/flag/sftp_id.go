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
	command.Flags().StringVarP(&sftpIDFlagValues.IdentityFilePath, "identity_file", "i", "", "Specify identity file path")
}

func GetSFTPIDFlagValues() *SFTPIDFlagValues {
	return &sftpIDFlagValues
}

package flag

import (
	"github.com/spf13/cobra"
)

type ChecksumFlagValues struct {
	VerifyChecksum    bool
	CalculateChecksum bool
}

var (
	checksumFlagValues ChecksumFlagValues
)

func SetChecksumFlags(command *cobra.Command, addCalculateChecksumFlag bool) {
	if addCalculateChecksumFlag {
		command.Flags().BoolVarP(&checksumFlagValues.CalculateChecksum, "checksum", "k", false, "Calculate a checksum on the data server-side")
	}

	command.Flags().BoolVarP(&checksumFlagValues.VerifyChecksum, "verify_checksum", "K", false, "calculate and verify the checksum")
}

func GetChecksumFlagValues() *ChecksumFlagValues {
	return &checksumFlagValues
}

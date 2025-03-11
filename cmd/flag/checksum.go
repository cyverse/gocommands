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

func SetChecksumFlags(command *cobra.Command, hideCalculateChecksum bool, hideVerifyChecksum bool) {
	command.Flags().BoolVarP(&checksumFlagValues.CalculateChecksum, "checksum", "k", false, "Calculate a checksum on the data server-side")
	command.Flags().BoolVarP(&checksumFlagValues.VerifyChecksum, "verify_checksum", "K", false, "Calculate and verify the checksum after transfer")

	if hideCalculateChecksum {
		command.Flags().MarkHidden("checksum")
	}

	if hideVerifyChecksum {
		command.Flags().MarkHidden("verify_checksum")
	}
}

func GetChecksumFlagValues() *ChecksumFlagValues {
	return &checksumFlagValues
}

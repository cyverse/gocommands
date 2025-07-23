package flag

import (
	"github.com/cyverse/gocommands/commons/config"
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
	command.Flags().BoolVarP(&checksumFlagValues.CalculateChecksum, "checksum", "k", config.GetDefaultVerifyChecksum(), "Generate checksum on the server side after data upload")
	command.Flags().BoolVarP(&checksumFlagValues.VerifyChecksum, "verify_checksum", "K", config.GetDefaultVerifyChecksum(), "Calculate and verify checksums to ensure data integrity after transfer")

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

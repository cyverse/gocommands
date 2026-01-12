package flag

import (
	"github.com/cyverse/gocommands/commons/config"
	"github.com/spf13/cobra"
)

type ChecksumFlagValues struct {
	VerifyChecksum bool
}

var (
	checksumFlagValues ChecksumFlagValues
)

func SetChecksumFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&checksumFlagValues.VerifyChecksum, "verify_checksum", "k", config.GetDefaultVerifyChecksum(), "Calculate and verify checksums to ensure data integrity after transfer")
}

func GetChecksumFlagValues() *ChecksumFlagValues {
	return &checksumFlagValues
}

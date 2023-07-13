package flag

import (
	"github.com/spf13/cobra"
)

type DifferentialTransferFlagValues struct {
	DifferentialTransfer bool
	NoHash               bool
}

var (
	differentialTransferFlagValues DifferentialTransferFlagValues
)

func SetDifferentialTransferFlags(command *cobra.Command, defaultDiff bool) {
	if defaultDiff {
		command.Flags().BoolVar(&differentialTransferFlagValues.DifferentialTransfer, "diff", false, "Transfer files with different content")
	} else {
		differentialTransferFlagValues.DifferentialTransfer = true
	}

	command.Flags().BoolVar(&differentialTransferFlagValues.NoHash, "no_hash", false, "Compare files without using hash")
}

func GetDifferentialTransferFlagValues() *DifferentialTransferFlagValues {
	return &differentialTransferFlagValues
}

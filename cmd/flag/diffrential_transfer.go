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

func SetDifferentialTransferFlags(command *cobra.Command, hideDiff bool) {
	command.Flags().BoolVar(&differentialTransferFlagValues.DifferentialTransfer, "diff", false, "Transfer files with different content")
	command.Flags().BoolVar(&differentialTransferFlagValues.NoHash, "no_hash", false, "Compare files without using hash, works with '--diff'")

	if hideDiff {
		command.Flags().MarkHidden("diff")
	}
}

func GetDifferentialTransferFlagValues() *DifferentialTransferFlagValues {
	return &differentialTransferFlagValues
}

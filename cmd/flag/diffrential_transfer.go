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
	command.Flags().BoolVar(&differentialTransferFlagValues.DifferentialTransfer, "diff", false, "Only transfer files that have different content than existing destination files")
	command.Flags().BoolVar(&differentialTransferFlagValues.NoHash, "no_hash", false, "Use file size and modification time instead of hash for file comparison when using '--diff'")

	if hideDiff {
		command.Flags().MarkHidden("diff")
	}
}

func GetDifferentialTransferFlagValues() *DifferentialTransferFlagValues {
	return &differentialTransferFlagValues
}

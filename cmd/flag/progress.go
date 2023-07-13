package flag

import (
	"github.com/spf13/cobra"
)

type ProgressFlagValues struct {
	ShowProgress bool
}

var (
	progressFlagValues ProgressFlagValues
)

func SetProgressFlags(command *cobra.Command) {
	command.Flags().BoolVar(&progressFlagValues.ShowProgress, "progress", false, "Display progress bars")
}

func GetProgressFlagValues() *ProgressFlagValues {
	return &progressFlagValues
}

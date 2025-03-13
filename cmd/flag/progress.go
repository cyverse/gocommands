package flag

import (
	"github.com/spf13/cobra"
)

type ProgressFlagValues struct {
	ShowProgress bool
	ShowFullPath bool
}

var (
	progressFlagValues ProgressFlagValues
)

func SetProgressFlags(command *cobra.Command) {
	command.Flags().BoolVar(&progressFlagValues.ShowProgress, "progress", false, "Show progress bars during transfer")
	command.Flags().BoolVar(&progressFlagValues.ShowFullPath, "show_path", false, "Show full file paths in progress bars")
}

func GetProgressFlagValues() *ProgressFlagValues {
	return &progressFlagValues
}

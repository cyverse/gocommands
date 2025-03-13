package flag

import (
	"github.com/spf13/cobra"
)

type HiddenFileFlagValues struct {
	Exclude bool
}

var (
	hiddenFileFlagValues HiddenFileFlagValues
)

func SetHiddenFileFlags(command *cobra.Command) {
	command.Flags().BoolVar(&hiddenFileFlagValues.Exclude, "exclude_hidden_files", false, "Skip files and directories that start with '.'")
}

func GetHiddenFileFlagValues() *HiddenFileFlagValues {
	return &hiddenFileFlagValues
}

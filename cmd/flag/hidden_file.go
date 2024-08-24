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
	command.Flags().BoolVar(&hiddenFileFlagValues.Exclude, "exclude_hidden_files", false, "Exclude hidden files (starting with '.')")
}

func GetHiddenFileFlagValues() *HiddenFileFlagValues {
	return &hiddenFileFlagValues
}

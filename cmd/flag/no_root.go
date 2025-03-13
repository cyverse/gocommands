package flag

import (
	"github.com/spf13/cobra"
)

type NoRootFlagValues struct {
	NoRoot bool
}

var (
	noRootFlagValues NoRootFlagValues
)

func SetNoRootFlags(command *cobra.Command) {
	command.Flags().BoolVar(&noRootFlagValues.NoRoot, "no_root", false, "Avoid creating the root directory at the destination during operation")
}

func GetNoRootFlagValues() *NoRootFlagValues {
	return &noRootFlagValues
}

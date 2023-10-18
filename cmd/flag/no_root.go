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
	command.Flags().BoolVar(&noRootFlagValues.NoRoot, "no_root", false, "Do not create root directory")
}

func GetNoRootFlagValues() *NoRootFlagValues {
	return &noRootFlagValues
}

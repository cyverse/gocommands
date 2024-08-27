package flag

import (
	"github.com/spf13/cobra"
)

type NoCreateFlagValues struct {
	NoCreate bool
}

var (
	noCreateFlagValues NoCreateFlagValues
)

func SetNoCreateFlags(command *cobra.Command) {
	command.Flags().BoolVar(&noCreateFlagValues.NoCreate, "no_create", false, "Do not create data object")
}

func GetNoCreateFlagValues() *NoCreateFlagValues {
	return &noCreateFlagValues
}

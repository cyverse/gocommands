package flag

import (
	"github.com/spf13/cobra"
)

type RecursiveFlagValues struct {
	Recursive bool
}

var (
	recursiveFlagValues RecursiveFlagValues
)

func SetRecursiveFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&recursiveFlagValues.Recursive, "recursive", "r", false, "Run recursively")
}

func GetRecursiveFlagValues() *RecursiveFlagValues {
	return &recursiveFlagValues
}

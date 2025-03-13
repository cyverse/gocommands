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

func SetRecursiveFlags(command *cobra.Command, hideRecursive bool) {
	command.Flags().BoolVarP(&recursiveFlagValues.Recursive, "recursive", "r", false, "Recursively process operations for collections and their contents")

	if hideRecursive {
		command.Flags().MarkHidden("recursive")
	}
}

func GetRecursiveFlagValues() *RecursiveFlagValues {
	return &recursiveFlagValues
}

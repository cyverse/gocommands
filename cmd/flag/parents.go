package flag

import (
	"github.com/spf13/cobra"
)

type ParentsFlagValues struct {
	MakeParents bool
}

var (
	parentsFlagValues ParentsFlagValues
)

func SetParentsFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&parentsFlagValues.MakeParents, "parents", "p", false, "Make parent collections")

}

func GetParentsFlagValues() *ParentsFlagValues {
	return &parentsFlagValues
}

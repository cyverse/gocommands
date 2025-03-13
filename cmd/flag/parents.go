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
	command.Flags().BoolVarP(&parentsFlagValues.MakeParents, "parents", "p", false, "Create parent collections if they do not exist")

}

func GetParentsFlagValues() *ParentsFlagValues {
	return &parentsFlagValues
}

package flag

import (
	"github.com/spf13/cobra"
)

type TargetObjectFlagValues struct {
	Path     bool
	Resource bool
	User     bool
}

var (
	targetObjectFlagValues TargetObjectFlagValues
)

func SetTargetObjectFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&targetObjectFlagValues.Path, "path", "P", false, "Indicate that the target is a data object or collection path")
	command.Flags().BoolVarP(&targetObjectFlagValues.Resource, "resource", "R", false, "Indicate that the target is a resource as the target")
	command.Flags().BoolVarP(&targetObjectFlagValues.User, "user", "U", false, "Indicate that the target is a user")

	command.MarkFlagsMutuallyExclusive("path", "resource", "user")
}

func GetTargetObjectFlagValues(command *cobra.Command) *TargetObjectFlagValues {
	if !targetObjectFlagValues.Path && !targetObjectFlagValues.Resource && !targetObjectFlagValues.User {
		targetObjectFlagValues.Path = true
	}

	return &targetObjectFlagValues
}

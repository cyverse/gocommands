package flag

import (
	"github.com/spf13/cobra"
)

type TargetObjectFlagValues struct {
	PathUpdated     bool
	Path            string
	ResourceUpdated bool
	Resource        string
	UserUpdated     bool
	User            string
}

var (
	targetObjectFlagValues TargetObjectFlagValues
)

func SetTargetObjectFlags(command *cobra.Command) {
	command.Flags().StringVarP(&targetObjectFlagValues.Path, "path", "P", "", "Specify a data object or collection as the target")
	command.Flags().StringVarP(&targetObjectFlagValues.Resource, "resource", "R", "", "Specify a resource as the target")
	command.Flags().StringVarP(&targetObjectFlagValues.User, "user", "U", "", "Specify a user as the target")

	command.MarkFlagsMutuallyExclusive("path", "resource", "user")
}

func GetTargetObjectFlagValues(command *cobra.Command) *TargetObjectFlagValues {
	if command.Flags().Changed("path") {
		targetObjectFlagValues.PathUpdated = true
	}

	if command.Flags().Changed("resource") {
		targetObjectFlagValues.ResourceUpdated = true
	}

	if command.Flags().Changed("user") {
		targetObjectFlagValues.UserUpdated = true
	}

	return &targetObjectFlagValues
}

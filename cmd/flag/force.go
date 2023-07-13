package flag

import (
	"github.com/spf13/cobra"
)

type ForceFlagValues struct {
	Force bool
}

var (
	forceFlagValues ForceFlagValues
)

func SetForceFlags(command *cobra.Command, hiddenForce bool) {
	command.Flags().BoolVarP(&forceFlagValues.Force, "force", "f", false, "Run forcefully")

	if hiddenForce {
		command.Flags().MarkHidden("force")
	}
}

func GetForceFlagValues() *ForceFlagValues {
	return &forceFlagValues
}

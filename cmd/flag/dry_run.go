package flag

import (
	"github.com/spf13/cobra"
)

type DryRunFlagValues struct {
	DryRun bool
}

var (
	dryRunFlagValues DryRunFlagValues
)

func SetDryRunFlags(command *cobra.Command) {
	command.Flags().BoolVar(&dryRunFlagValues.DryRun, "dry_run", false, "Simulate execution without making actual changes")
}

func GetDryRunFlagValues() *DryRunFlagValues {
	return &dryRunFlagValues
}

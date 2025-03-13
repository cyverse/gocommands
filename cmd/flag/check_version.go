package flag

import (
	"github.com/spf13/cobra"
)

type CheckVersionFlagValues struct {
	Check bool
}

var (
	checkVersionFlagValues CheckVersionFlagValues
)

func SetCheckVersionFlags(command *cobra.Command) {
	command.Flags().BoolVar(&checkVersionFlagValues.Check, "check", false, "Only check for the latest version without performing any updates")
}

func GetCheckVersionFlagValues() *CheckVersionFlagValues {
	return &checkVersionFlagValues
}

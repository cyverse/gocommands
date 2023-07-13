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
	command.Flags().BoolVar(&checkVersionFlagValues.Check, "check", false, "Check the latest version only")
}

func GetCheckVersionFlagValues() *CheckVersionFlagValues {
	return &checkVersionFlagValues
}

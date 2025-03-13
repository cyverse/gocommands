package flag

import (
	"github.com/spf13/cobra"
)

type InitFlagValues struct {
	PamTTL int
}

var (
	initFlagValues InitFlagValues
)

func SetInitFlags(command *cobra.Command) {
	command.Flags().IntVar(&initFlagValues.PamTTL, "ttl", 0, "Set the password time-to-live in seconds")
}

func GetInitFlagValues() *InitFlagValues {
	return &initFlagValues
}

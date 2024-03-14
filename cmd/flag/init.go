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
	command.Flags().IntVar(&initFlagValues.PamTTL, "ttl", 1, "Set the password time to live")
}

func GetInitFlagValues() *InitFlagValues {
	return &initFlagValues
}

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
	command.Flags().IntVar(&initFlagValues.PamTTL, "ttl", 0, "Specify the authentication token's time-to-live (TTL) in hours for PAM authentication")
}

func GetInitFlagValues() *InitFlagValues {
	return &initFlagValues
}

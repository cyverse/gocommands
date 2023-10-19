package flag

import (
	"github.com/spf13/cobra"
)

type SyncFlagValues struct {
	Delete bool
}

var (
	syncFlagValues SyncFlagValues
)

func SetSyncFlags(command *cobra.Command) {
	command.Flags().BoolVar(&syncFlagValues.Delete, "delete", false, "Delete extra files in dest dir")
}

func GetSyncFlagValues() *SyncFlagValues {
	return &syncFlagValues
}

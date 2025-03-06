package flag

import (
	"github.com/spf13/cobra"
)

type SyncFlagValues struct {
	Delete     bool
	BulkUpload bool
	Age        int
	Sync       bool
}

var (
	syncFlagValues SyncFlagValues
)

func SetSyncFlags(command *cobra.Command, hideBulkUpload bool) {
	command.Flags().BoolVar(&syncFlagValues.Delete, "delete", false, "Delete extra files in dest dir")
	command.Flags().BoolVar(&syncFlagValues.BulkUpload, "bulk_upload", false, "Use bulk upload")
	command.Flags().BoolVar(&syncFlagValues.Sync, "sync", false, "Set this for sync")
	command.Flags().IntVar(&syncFlagValues.Age, "age", 0, "Set the maximum age of the source in minutes")
	command.Flags().MarkHidden("sync")

	if hideBulkUpload {
		command.Flags().MarkHidden("bulk_upload")
	}
}

func GetSyncFlagValues() *SyncFlagValues {
	return &syncFlagValues
}

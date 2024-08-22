package flag

import (
	"github.com/spf13/cobra"
)

type SyncFlagValues struct {
	Delete     bool
	BulkUpload bool
	Sync       bool
}

var (
	syncFlagValues SyncFlagValues
)

func SetSyncFlags(command *cobra.Command, addPutOptionFlag bool) {
	command.Flags().BoolVar(&syncFlagValues.Delete, "delete", false, "Delete extra files in dest dir")

	if addPutOptionFlag {
		command.Flags().BoolVar(&syncFlagValues.BulkUpload, "bulk_upload", false, "Use bulk upload")
	}

	// this is hidden
	command.Flags().BoolVar(&syncFlagValues.Sync, "sync", false, "Set this for sync")
	command.Flags().MarkHidden("sync")
}

func GetSyncFlagValues() *SyncFlagValues {
	return &syncFlagValues
}

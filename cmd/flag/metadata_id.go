package flag

import (
	"github.com/spf13/cobra"
)

type MetadataByIDFlagValues struct {
	ByID bool
}

var (
	metadataByID MetadataByIDFlagValues
)

func SetMetadataByIDFlags(command *cobra.Command) {
	command.Flags().BoolVar(&metadataByID.ByID, "id", false, "Specify metadata ID instead of AVU")
}

func GetMetadataByIDFlagValues(command *cobra.Command) *MetadataByIDFlagValues {
	return &metadataByID
}

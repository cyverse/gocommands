package flag

import (
	"github.com/spf13/cobra"
)

type PostTransferFlagValues struct {
	DeleteOnSuccess bool
}

var (
	postTransferFlagValues PostTransferFlagValues
)

func SetPostTransferFlagValues(command *cobra.Command) {
	command.Flags().BoolVar(&postTransferFlagValues.DeleteOnSuccess, "delete_on_success", false, "Delete source file on success")
}

func GetPostTransferFlagValues() *PostTransferFlagValues {
	return &postTransferFlagValues
}

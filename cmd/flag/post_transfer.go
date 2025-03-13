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
	command.Flags().BoolVar(&postTransferFlagValues.DeleteOnSuccess, "delete_on_success", false, "Delete the source file after a successful transfer")
}

func GetPostTransferFlagValues() *PostTransferFlagValues {
	return &postTransferFlagValues
}

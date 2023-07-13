package flag

import (
	"github.com/spf13/cobra"
)

type ListFormat string

const (
	ListFormatNormal   ListFormat = ""
	ListFormatLong     ListFormat = "long"
	ListFormatVeryLong ListFormat = "verylong"
)

type ListFlagValues struct {
	Format              ListFormat
	longFormatInput     bool
	veryLongFormatInput bool
}

var (
	listFlagValues ListFlagValues
)

func SetListFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display in a long format")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display in a very long format")
}

func GetListFlagValues() *ListFlagValues {
	if listFlagValues.veryLongFormatInput {
		listFlagValues.Format = ListFormatVeryLong
	} else if listFlagValues.longFormatInput {
		listFlagValues.Format = ListFormatLong
	} else {
		listFlagValues.Format = ListFormatNormal
	}

	return &listFlagValues
}

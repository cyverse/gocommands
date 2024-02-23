package flag

import (
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type ListFlagValues struct {
	Format              commons.ListFormat
	longFormatInput     bool
	veryLongFormatInput bool
	HumanReadableSizes  bool

	SortOrder      commons.ListSortOrder
	sortOrderInput string
	SortReverse    bool
}

var (
	listFlagValues ListFlagValues
)

func SetListFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display in a long format")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display in a very long format")
	command.Flags().BoolVarP(&listFlagValues.HumanReadableSizes, "human_readable", "H", false, "Display sizes in human-readable format")
	command.Flags().BoolVar(&listFlagValues.SortReverse, "reverse_sort", false, "Sort in reverse order")
	command.Flags().StringVarP(&listFlagValues.sortOrderInput, "sort", "name", "S", "Sort on name, size, time or ext")
}

func GetListFlagValues() *ListFlagValues {
	if listFlagValues.veryLongFormatInput {
		listFlagValues.Format = commons.ListFormatVeryLong
	} else if listFlagValues.longFormatInput {
		listFlagValues.Format = commons.ListFormatLong
	} else {
		listFlagValues.Format = commons.ListFormatNormal
	}

	listFlagValues.SortOrder = commons.GetListSortOrder(listFlagValues.sortOrderInput)

	return &listFlagValues
}

package flag

import (
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type ListFlagValues struct {
	Format              commons.ListFormat
	longFormatInput     bool
	veryLongFormatInput bool
	Access              bool
	HumanReadableSizes  bool

	SortOrder      commons.ListSortOrder
	sortOrderInput string
	SortReverse    bool
}

var (
	listFlagValues ListFlagValues
)

func SetListFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display results in long format with additional details")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display results in very long format with comprehensive information")
	command.Flags().BoolVarP(&listFlagValues.HumanReadableSizes, "human_readable", "H", false, "Show file sizes in human-readable units (KB, MB, GB)")
	command.Flags().BoolVarP(&listFlagValues.Access, "access", "A", false, "Display access control lists for data-objects and collections")
	command.Flags().BoolVar(&listFlagValues.SortReverse, "reverse_sort", false, "Sort results in reverse order")
	command.Flags().StringVarP(&listFlagValues.sortOrderInput, "sort", "S", "name", "Sort results by: name, size, time, or ext")
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

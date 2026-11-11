package flag

import (
	"github.com/cyverse/gocommands/commons/format"
	"github.com/spf13/cobra"
)

type ListFlagValues struct {
	Format              format.ListFormat
	longFormatInput     bool
	veryLongFormatInput bool
	Access              bool
	HumanReadableSizes  bool
	QuoteNames          bool

	SortOrder      format.ListSortOrder
	sortOrderInput string
	SortReverse    bool
}

var (
	listFlagValues ListFlagValues
)

func SetListFlags(command *cobra.Command, hideHumanReadable bool, hideAccess bool, hideQuote bool) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display results in long format with additional details")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display results in very long format with comprehensive information")
	command.Flags().BoolVarP(&listFlagValues.HumanReadableSizes, "human_readable", "H", false, "Show file sizes in human-readable units (KB, MB, GB)")
	command.Flags().BoolVarP(&listFlagValues.Access, "access", "A", false, "Display access control lists for data-objects and collections")
	command.Flags().BoolVar(&listFlagValues.SortReverse, "reverse_sort", false, "Sort results in reverse order")
	command.Flags().StringVarP(&listFlagValues.sortOrderInput, "sort", "S", "name", "Sort results by: name, size, time, or ext")
	command.Flags().BoolVarP(&listFlagValues.QuoteNames, "quote_names", "Q", false, "Quote data object and collection names in C style")

	if hideHumanReadable {
		command.Flags().MarkHidden("human_readable")
	}

	if hideAccess {
		command.Flags().MarkHidden("access")
	}

	if hideQuote {
		command.Flags().MarkHidden("quote_names")
	}
}

func SetListFlagsForHumanReadableSizes(command *cobra.Command) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display results in long format with additional details")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display results in very long format with comprehensive information")
	command.Flags().BoolVarP(&listFlagValues.HumanReadableSizes, "human_readable", "H", false, "Show file sizes in human-readable units (KB, MB, GB)")
	command.Flags().BoolVarP(&listFlagValues.Access, "access", "A", false, "Display access control lists for data-objects and collections")
	command.Flags().BoolVar(&listFlagValues.SortReverse, "reverse_sort", false, "Sort results in reverse order")
	command.Flags().StringVarP(&listFlagValues.sortOrderInput, "sort", "S", "name", "Sort results by: name, size, time, or ext")

	command.Flags().MarkHidden("long")
	command.Flags().MarkHidden("verylong")
	command.Flags().MarkHidden("access")
	command.Flags().MarkHidden("reverse_sort")
	command.Flags().MarkHidden("sort")
}

func GetListFlagValues() *ListFlagValues {
	if listFlagValues.veryLongFormatInput {
		listFlagValues.Format = format.ListFormatVeryLong
	} else if listFlagValues.longFormatInput {
		listFlagValues.Format = format.ListFormatLong
	} else {
		listFlagValues.Format = format.ListFormatNormal
	}

	listFlagValues.SortOrder = format.GetListSortOrder(listFlagValues.sortOrderInput)

	return &listFlagValues
}

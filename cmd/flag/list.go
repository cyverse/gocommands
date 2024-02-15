package flag

import (
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

type ListFormat string
type SortOrder string

const (
	ListFormatNormal   ListFormat = ""
	ListFormatLong     ListFormat = "long"
	ListFormatVeryLong ListFormat = "verylong"
	SortOrderName      SortOrder  = "name"
	SortOrderSize      SortOrder  = "size"
	SortOrderTime      SortOrder  = "time"
	SortOrderExt       SortOrder  = "ext"
)

type ListFlagValues struct {
	Format              ListFormat
	longFormatInput     bool
	veryLongFormatInput bool
	HumanReadableSizes  bool
}

type SortOrderConfig struct {
	SortOrder       SortOrder
	sortOrderString string
	ReverseSort     bool
}

var (
	listFlagValues  ListFlagValues
	sortOrderConfig SortOrderConfig
)

func ValidateListFlags(cmd *cobra.Command, args []string) error {
	sFlagSet := cmd.Flags().Changed("sort")
	if !sFlagSet {
		return nil
	}

	sortOrder, err := cmd.Flags().GetString("sort")
	if err != nil {
		return err
	}

	switch sortOrder {
	case string(SortOrderName), string(SortOrderSize), string(SortOrderTime), string(SortOrderExt):
		return nil
	default:
		return xerrors.New("Invalid sort order. Expect one of: name, size, time, ext.")
	}

}

func SetListFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&listFlagValues.longFormatInput, "long", "l", false, "Display in a long format")
	command.Flags().BoolVarP(&listFlagValues.veryLongFormatInput, "verylong", "L", false, "Display in a very long format")
	command.Flags().BoolVarP(&listFlagValues.HumanReadableSizes, "human_readable", "H", false, "Display sizes in human-readable format")
	command.Flags().BoolVarP(&sortOrderConfig.ReverseSort, "reverse-sort", "r", false, "Reverse sort order")
	command.Flags().StringVarP(&sortOrderConfig.sortOrderString, "sort", "S", "name", "Sort on: name, size, time or ext")
	command.PreRunE = ValidateListFlags
	sortOrderConfig.SortOrder = SortOrder(sortOrderConfig.sortOrderString)
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

func GetSortOrderConfig() *SortOrderConfig {
	sortOrderConfig.SortOrder = SortOrder(sortOrderConfig.sortOrderString)
	return &sortOrderConfig
}

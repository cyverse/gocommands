package flag

import (
	"github.com/spf13/cobra"
)

type WildcardSearchFlagValues struct {
	WildcardSearch bool
}

var (
	wildcardSearchFlagValues WildcardSearchFlagValues
)

func SetWildcardSearchFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&wildcardSearchFlagValues.WildcardSearch, "wildcard", "w", false, "Enable wildcard expansion to search source files")
}

func GetWildcardSearchFlagValues() *WildcardSearchFlagValues {
	return &wildcardSearchFlagValues
}

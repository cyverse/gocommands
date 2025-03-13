package flag

import "github.com/spf13/cobra"

type BundleFlagValues struct {
	Extract          bool
	BulkRegistration bool
	DataType         string
}

var (
	bundleFlagValues BundleFlagValues
)

func SetBundleFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&bundleFlagValues.Extract, "extract", "x", false, "Extract contents from the bundle file")
	command.Flags().BoolVarP(&bundleFlagValues.BulkRegistration, "bulk", "b", false, "Enable bulk registration mode for processing multiple items simultaneously")
	command.Flags().StringVarP(&bundleFlagValues.DataType, "data_type", "D", "", "Specify archive format type (e.g., tar, zip, gz) for processing")
}

func GetBundleFlagValues() *BundleFlagValues {
	return &bundleFlagValues
}

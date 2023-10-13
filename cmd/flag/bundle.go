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
	command.Flags().BoolVarP(&bundleFlagValues.Extract, "extract", "x", false, "Extract")
	command.Flags().BoolVarP(&bundleFlagValues.BulkRegistration, "bulk", "b", false, "Enable bulk registration")
	command.Flags().StringVarP(&bundleFlagValues.DataType, "data_type", "D", "", "Set data type (tar, zip ...)")
}

func GetBundleFlagValues() *BundleFlagValues {
	return &bundleFlagValues
}

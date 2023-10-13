package flag

import (
	"os"
	"strconv"

	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type BundleTempFlagValues struct {
	LocalTempPath string
	IRODSTempPath string
}

type BundleClearFlagVlaues struct {
	Clear bool
}

type BundleConfigFlagValues struct {
	MaxFileNum         int
	MaxFileSize        int64
	NoBulkRegistration bool
	maxFileSizeInput   string
}

var (
	bundleTempFlagValues   BundleTempFlagValues
	bundleClearFlagValues  BundleClearFlagVlaues
	bundleConfigFlagValues BundleConfigFlagValues
)

func SetBundleTempFlags(command *cobra.Command) {
	command.Flags().StringVar(&bundleTempFlagValues.LocalTempPath, "local_temp", os.TempDir(), "Specify local temp directory path to create bundle files")
	command.Flags().StringVar(&bundleTempFlagValues.IRODSTempPath, "irods_temp", "", "Specify iRODS temp collection path to upload bundle files to")
}

func GetBundleTempFlagValues() *BundleTempFlagValues {
	return &bundleTempFlagValues
}

func SetBundleClearFlags(command *cobra.Command) {
	command.Flags().BoolVar(&bundleClearFlagValues.Clear, "clear", false, "Clear stale bundle files")
}

func GetBundleClearFlagValues() *BundleClearFlagVlaues {
	return &bundleClearFlagValues
}

func SetBundleConfigFlags(command *cobra.Command) {
	command.Flags().IntVar(&bundleConfigFlagValues.MaxFileNum, "max_file_num", commons.MaxBundleFileNumDefault, "Specify max file number in a bundle file")
	command.Flags().StringVar(&bundleConfigFlagValues.maxFileSizeInput, "max_file_size", strconv.FormatInt(commons.MaxBundleFileSizeDefault, 10), "Specify max file size of a bundle file")
	command.Flags().BoolVar(&bundleConfigFlagValues.NoBulkRegistration, "no_bulk_reg", false, "Disable bulk registration")
}

func GetBundleConfigFlagValues() *BundleConfigFlagValues {
	size, _ := commons.ParseSize(bundleConfigFlagValues.maxFileSizeInput)
	bundleConfigFlagValues.MaxFileSize = size

	return &bundleConfigFlagValues
}

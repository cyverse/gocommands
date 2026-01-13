package flag

import (
	"github.com/cyverse/gocommands/commons/format"
	"github.com/spf13/cobra"
)

type OutputFormatFlagValues struct {
	Format         format.OutputFormat
	csvFormatInput bool
	tsvFormatInput bool
}

var (
	outputFormatFlagValues OutputFormatFlagValues
)

func SetOutputFormatFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&outputFormatFlagValues.csvFormatInput, "output_csv", "", false, "Display results in CSV format")
	command.Flags().BoolVarP(&outputFormatFlagValues.tsvFormatInput, "output_tsv", "", false, "Display results in TSV format")
}

func GetOutputFormatFlagValues() *OutputFormatFlagValues {
	if outputFormatFlagValues.csvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatCSV
	} else if outputFormatFlagValues.tsvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatTSV
	} else {
		outputFormatFlagValues.Format = format.OutputFormatTable
	}

	return &outputFormatFlagValues
}

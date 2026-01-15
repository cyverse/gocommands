package flag

import (
	"github.com/cyverse/gocommands/commons/format"
	"github.com/spf13/cobra"
)

type OutputFormatFlagValues struct {
	Format          format.OutputFormat
	csvFormatInput  bool
	tsvFormatInput  bool
	jsonFormatInput bool
}

var (
	outputFormatFlagValues OutputFormatFlagValues
)

func SetOutputFormatFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&outputFormatFlagValues.csvFormatInput, "output_csv", "", false, "Display results in CSV format")
	command.Flags().BoolVarP(&outputFormatFlagValues.tsvFormatInput, "output_tsv", "", false, "Display results in TSV format")
	command.Flags().BoolVarP(&outputFormatFlagValues.jsonFormatInput, "output_json", "", false, "Display results in JSON format")
}

func GetOutputFormatFlagValues() *OutputFormatFlagValues {
	if outputFormatFlagValues.csvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatCSV
	} else if outputFormatFlagValues.tsvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatTSV
	} else if outputFormatFlagValues.jsonFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatJSON
	} else {
		outputFormatFlagValues.Format = format.OutputFormatTable
	}

	return &outputFormatFlagValues
}

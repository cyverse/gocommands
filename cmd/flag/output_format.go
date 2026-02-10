package flag

import (
	"github.com/cyverse/gocommands/commons/format"
	"github.com/spf13/cobra"
)

type OutputFormatFlagValues struct {
	Format            format.OutputFormat
	csvFormatInput    bool
	tsvFormatInput    bool
	jsonFormatInput   bool
	legacyFormatInput bool
}

var (
	outputFormatFlagValues OutputFormatFlagValues
)

func SetOutputFormatFlags(command *cobra.Command, hideLegacy bool) {
	command.Flags().BoolVarP(&outputFormatFlagValues.csvFormatInput, "output_csv", "", false, "Display results in CSV format")
	command.Flags().BoolVarP(&outputFormatFlagValues.tsvFormatInput, "output_tsv", "", false, "Display results in TSV format")
	command.Flags().BoolVarP(&outputFormatFlagValues.jsonFormatInput, "output_json", "", false, "Display results in JSON format")
	command.Flags().BoolVarP(&outputFormatFlagValues.legacyFormatInput, "output_legacy", "", false, "Display results in old-iCommands format")

	if hideLegacy {
		command.Flags().MarkHidden("output_legacy")
	}
}

func GetOutputFormatFlagValues() *OutputFormatFlagValues {
	if outputFormatFlagValues.csvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatCSV
	} else if outputFormatFlagValues.tsvFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatTSV
	} else if outputFormatFlagValues.jsonFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatJSON
	} else if outputFormatFlagValues.legacyFormatInput {
		outputFormatFlagValues.Format = format.OutputFormatLegacy
	} else {
		outputFormatFlagValues.Format = format.OutputFormatTable
	}

	return &outputFormatFlagValues
}

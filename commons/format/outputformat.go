package format

import "strings"

type OutputFormat string

const (
	// format
	OutputFormatTable OutputFormat = "table"
	OutputFormatTSV   OutputFormat = "tsv"
	OutputFormatCSV   OutputFormat = "csv"
)

// GetOutputFormat returns OutputFormat from string
func GetOutputFormat(order string) OutputFormat {
	switch strings.ToLower(order) {
	case string(OutputFormatTable):
		return OutputFormatTable
	case string(OutputFormatTSV):
		return OutputFormatTSV
	case string(OutputFormatCSV):
		return OutputFormatCSV
	default:
		return OutputFormatTable
	}
}

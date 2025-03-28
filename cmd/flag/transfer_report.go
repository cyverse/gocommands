package flag

import (
	"github.com/spf13/cobra"
)

type TransferReportFlagValues struct {
	ReportPath     string
	Report         bool
	ReportToStdout bool
}

var (
	transferReportFlagValues TransferReportFlagValues
)

func SetTransferReportFlags(command *cobra.Command) {
	command.Flags().StringVar(&transferReportFlagValues.ReportPath, "report", "", "Create a transfer report; specify the path for file output. An empty string or '-' outputs to stdout")
}

func GetTransferReportFlagValues(command *cobra.Command) *TransferReportFlagValues {
	if command.Flags().Changed("report") {
		transferReportFlagValues.Report = true
	}

	transferReportFlagValues.ReportToStdout = false

	if transferReportFlagValues.ReportPath == "-" || len(transferReportFlagValues.ReportPath) == 0 {
		transferReportFlagValues.ReportToStdout = true
	}

	return &transferReportFlagValues
}

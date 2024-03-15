package flag

import (
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type ParallelTransferFlagValues struct {
	SingleTread        bool
	ThreadNumber       int
	TCPBufferSize      int
	tcpBufferSizeInput string
	RedirectToResource bool
	Icat               bool
}

var (
	parallelTransferFlagValues ParallelTransferFlagValues
)

func SetParallelTransferFlags(command *cobra.Command, showSingleThread bool) {
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumber, "thread_num", commons.TransferTreadNumDefault, "Specify the number of transfer threads")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpBufferSizeInput, "tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	command.Flags().BoolVar(&parallelTransferFlagValues.RedirectToResource, "redirect", false, "Always redirect to resource server")
	command.Flags().BoolVar(&parallelTransferFlagValues.Icat, "icat", false, "Always transfer data via iCAT")

	if showSingleThread {
		command.Flags().BoolVar(&parallelTransferFlagValues.SingleTread, "single_threaded", false, "Transfer a file using a single thread")
		command.MarkFlagsMutuallyExclusive("redirect", "single_threaded")
	}

	command.MarkFlagsMutuallyExclusive("redirect", "icat")
}

func GetParallelTransferFlagValues() *ParallelTransferFlagValues {
	size, _ := commons.ParseSize(parallelTransferFlagValues.tcpBufferSizeInput)
	parallelTransferFlagValues.TCPBufferSize = int(size)

	return &parallelTransferFlagValues
}

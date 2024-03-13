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
}

var (
	parallelTransferFlagValues ParallelTransferFlagValues
)

func SetParallelTransferFlags(command *cobra.Command, showSingleThread bool) {
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumber, "thread_num", commons.TransferTreadNumDefault, "Specify the number of transfer threads")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpBufferSizeInput, "tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	command.Flags().BoolVar(&parallelTransferFlagValues.RedirectToResource, "redirect", false, "Redirect to resource server")

	if showSingleThread {
		command.Flags().BoolVar(&parallelTransferFlagValues.SingleTread, "single_threaded", false, "Transfer a file using a single thread")
	}
}

func GetParallelTransferFlagValues() *ParallelTransferFlagValues {
	size, _ := commons.ParseSize(parallelTransferFlagValues.tcpBufferSizeInput)
	parallelTransferFlagValues.TCPBufferSize = int(size)

	return &parallelTransferFlagValues
}

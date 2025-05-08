package flag

import (
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
)

type ParallelTransferFlagValues struct {
	SingleThread        bool
	ThreadNumber        int
	ThreadNumberPerFile int
	TCPBufferSize       int
	tcpBufferSizeInput  string
	RedirectToResource  bool
	Icat                bool
}

var (
	parallelTransferFlagValues ParallelTransferFlagValues
)

func SetParallelTransferFlags(command *cobra.Command, hideParallelConfig bool, hideSingleThread bool) {
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumber, "thread_num", commons.GetDefaultTransferThreadNum(), "Set the total number of transfer threads")
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumberPerFile, "thread_num_per_file", commons.GetDefaultTransferThreadNum(), "Set the number of transfer threads for each file")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpBufferSizeInput, "tcp_buffer_size", commons.GetDefaultTCPBufferSizeString(), "Set the TCP socket buffer size")
	command.Flags().BoolVar(&parallelTransferFlagValues.RedirectToResource, "redirect", false, "Enable transfer redirection to the resource server")
	command.Flags().BoolVar(&parallelTransferFlagValues.Icat, "icat", false, "Use iCAT for file transfers")
	command.Flags().BoolVar(&parallelTransferFlagValues.SingleThread, "single_threaded", false, "Force single-threaded file transfer")

	if hideParallelConfig {
		command.Flags().MarkHidden("thread_num")
		command.Flags().MarkHidden("thread_num_per_file")
		command.Flags().MarkHidden("tcp_buffer_size")
		command.Flags().MarkHidden("redirect")
		command.Flags().MarkHidden("icat")
		command.Flags().MarkHidden("single_threaded")
	}

	if hideSingleThread {
		command.Flags().MarkHidden("single_threaded")
	}

	command.MarkFlagsMutuallyExclusive("redirect", "single_threaded")
	command.MarkFlagsMutuallyExclusive("redirect", "icat")
}

func GetParallelTransferFlagValues() *ParallelTransferFlagValues {
	size, _ := commons.ParseSize(parallelTransferFlagValues.tcpBufferSizeInput)
	parallelTransferFlagValues.TCPBufferSize = int(size)

	if parallelTransferFlagValues.ThreadNumber < 1 {
		parallelTransferFlagValues.ThreadNumber = 1
	}

	if parallelTransferFlagValues.ThreadNumberPerFile < 1 {
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	if parallelTransferFlagValues.ThreadNumber == 1 {
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	if parallelTransferFlagValues.SingleThread {
		parallelTransferFlagValues.ThreadNumber = 1
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	return &parallelTransferFlagValues
}

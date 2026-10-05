package flag

import (
	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/spf13/cobra"
)

type ParallelTransferFlagValues struct {
	SingleThread           bool
	ThreadNumber           int
	ThreadNumberPerFile    int
	TCPSendBufferSize      int
	TCPRecvBufferSize      int
	tcpSendBufferSizeInput string
	tcpRecvBufferSizeInput string
	Icat                   bool
	WebDAV                 bool
	RedirectToResource     bool
	StopOnError            bool
}

const maxTCPBufferSize = 100 * types.MegaBytes

var (
	parallelTransferFlagValues ParallelTransferFlagValues
)

func SetParallelTransferFlags(command *cobra.Command, hideParallelConfig bool, hideSingleThread bool) {
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumber, "thread_num", config.GetDefaultTransferThreadNum(), "Set the total number of transfer threads")
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumberPerFile, "thread_num_per_file", config.GetDefaultTransferThreadNumPerFile(), "Set the number of transfer threads for each file")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpSendBufferSizeInput, "tcp_send_buffer_size", config.GetDefaultTCPSendBufferSizeString(), "Set the TCP socket send buffer size")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpRecvBufferSizeInput, "tcp_recv_buffer_size", config.GetDefaultTCPRecvBufferSizeString(), "Set the TCP socket receive buffer size")
	command.Flags().BoolVar(&parallelTransferFlagValues.Icat, "icat", false, "Use iCAT for file transfers")
	command.Flags().BoolVar(&parallelTransferFlagValues.SingleThread, "single_threaded", false, "Force single-threaded file transfer")
	command.Flags().BoolVar(&parallelTransferFlagValues.WebDAV, "webdav", false, "Use WebDAV protocol (HTTP) for transfer")
	command.Flags().BoolVar(&parallelTransferFlagValues.RedirectToResource, "redirect", false, "Connect to resource servers directly for transfer")
	command.Flags().BoolVar(&parallelTransferFlagValues.StopOnError, "stop_on_error", false, "Stop all transfers immediately when an error occurs")

	if hideParallelConfig {
		command.Flags().MarkHidden("thread_num")
		command.Flags().MarkHidden("thread_num_per_file")
		command.Flags().MarkHidden("tcp_send_buffer_size")
		command.Flags().MarkHidden("tcp_recv_buffer_size")
		command.Flags().MarkHidden("icat")
		command.Flags().MarkHidden("single_threaded")
		command.Flags().MarkHidden("webdav")
		command.Flags().MarkHidden("redirect")
	}

	if hideSingleThread {
		command.Flags().MarkHidden("single_threaded")
	}

	command.MarkFlagsMutuallyExclusive("icat", "webdav", "redirect")
}

func GetParallelTransferFlagValues() (*ParallelTransferFlagValues, error) {
	sendSize, err := parseTCPBufferSize(parallelTransferFlagValues.tcpSendBufferSizeInput, "send")
	if err != nil {
		return nil, err
	}
	parallelTransferFlagValues.TCPSendBufferSize = sendSize

	recvSize, err := parseTCPBufferSize(parallelTransferFlagValues.tcpRecvBufferSizeInput, "receive")
	if err != nil {
		return nil, err
	}
	parallelTransferFlagValues.TCPRecvBufferSize = recvSize

	if parallelTransferFlagValues.ThreadNumber < 1 {
		parallelTransferFlagValues.ThreadNumber = 1
	}

	if parallelTransferFlagValues.ThreadNumberPerFile < 1 {
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	if parallelTransferFlagValues.ThreadNumber == 1 {
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	if parallelTransferFlagValues.ThreadNumberPerFile > parallelTransferFlagValues.ThreadNumber {
		parallelTransferFlagValues.ThreadNumberPerFile = parallelTransferFlagValues.ThreadNumber
	}

	if parallelTransferFlagValues.SingleThread {
		parallelTransferFlagValues.ThreadNumber = 1
		parallelTransferFlagValues.ThreadNumberPerFile = 1
	}

	return &parallelTransferFlagValues, nil
}

func parseTCPBufferSize(input string, direction string) (int, error) {
	size, err := types.ParseSize(input)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid tcp %s buffer size", direction)
	}
	if size > maxTCPBufferSize {
		return 0, errors.Errorf("tcp %s buffer size must not exceed %dMB", direction, maxTCPBufferSize/types.MegaBytes)
	}
	return int(size), nil
}

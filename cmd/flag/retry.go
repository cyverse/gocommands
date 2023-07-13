package flag

import (
	"github.com/spf13/cobra"
)

type RetryFlagValues struct {
	RetryNumber          int
	RetryIntervalSeconds int
	RetryChild           bool
}

var (
	retryFlagValues RetryFlagValues
)

func SetRetryFlags(command *cobra.Command) {
	command.Flags().IntVar(&retryFlagValues.RetryNumber, "retry", 1, "Retry if fails")
	command.Flags().IntVar(&retryFlagValues.RetryIntervalSeconds, "retry_interval", 60, "Retry interval in seconds")

	// this is hidden
	command.Flags().BoolVar(&retryFlagValues.RetryChild, "retry_child", false, "Set this to retry child process")
	command.Flags().MarkHidden("retry_child")
}

func GetRetryFlagValues() *RetryFlagValues {
	return &retryFlagValues
}

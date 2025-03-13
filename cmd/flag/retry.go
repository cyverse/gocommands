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
	command.Flags().IntVar(&retryFlagValues.RetryNumber, "retry", 0, "Set the number of retry attempts")
	command.Flags().IntVar(&retryFlagValues.RetryIntervalSeconds, "retry_interval", 60, "Set the interval between retry attempts in seconds")
	command.Flags().BoolVar(&retryFlagValues.RetryChild, "retry_child", false, "")

	command.Flags().MarkHidden("retry_child")
}

func GetRetryFlagValues() *RetryFlagValues {
	return &retryFlagValues
}

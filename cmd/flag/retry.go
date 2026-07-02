package flag

import (
	"time"

	"github.com/spf13/cobra"
)

const (
	DefaultRetryNumber          = 3
	DefaultRetryIntervalSeconds = 5
)

type RetryFlagValues struct {
	RetryNumber          int
	RetryIntervalSeconds int
}

var (
	retryFlagValues RetryFlagValues
)

func SetRetryFlags(command *cobra.Command) {
	command.Flags().IntVar(&retryFlagValues.RetryNumber, "retry", DefaultRetryNumber, "Set the number of retry attempts")
	command.Flags().IntVar(&retryFlagValues.RetryIntervalSeconds, "retry_interval", DefaultRetryIntervalSeconds, "Set the interval between retry attempts in seconds")
}

func GetRetryFlagValues() *RetryFlagValues {
	return &retryFlagValues
}

func (r *RetryFlagValues) GetRetryNumber() int {
	if r.RetryNumber < 0 {
		return DefaultRetryNumber
	}
	return r.RetryNumber
}

func (r *RetryFlagValues) GetRetryIntervalSeconds() time.Duration {
	if r.RetryIntervalSeconds <= 0 {
		return time.Duration(DefaultRetryIntervalSeconds) * time.Second
	}

	return time.Duration(r.RetryIntervalSeconds) * time.Second
}

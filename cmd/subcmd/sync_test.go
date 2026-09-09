package subcmd

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestFindCommandIndexSkipsFlagValues(t *testing.T) {
	flags := pflag.NewFlagSet("root", pflag.ContinueOnError)
	flags.StringP("config", "c", "", "")
	flags.BoolP("verbose", "v", false, "")

	testCases := []struct {
		name string
		args []string
		want int
	}{
		{name: "short flag value matches command", args: []string{"-c", "sync", "sync", "/a", "/b"}, want: 2},
		{name: "long inline flag value matches command", args: []string{"--config=sync", "sync", "/a", "/b"}, want: 1},
		{name: "boolean flag before command", args: []string{"-v", "sync", "/a", "/b"}, want: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := findCommandIndex(testCase.args, "sync", flags); got != testCase.want {
				t.Errorf("findCommandIndex() = %d, want %d", got, testCase.want)
			}
		})
	}
}

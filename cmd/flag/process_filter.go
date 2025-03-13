package flag

import (
	"github.com/spf13/cobra"
)

type ProcessGroupBy string

const (
	ProcessGroupByNone    ProcessGroupBy = ""
	ProcessGroupByUser    ProcessGroupBy = "user"
	ProcessGroupByProgram ProcessGroupBy = "program"
)

type ProcessFilterFlagValues struct {
	GroupBy             ProcessGroupBy
	groupByUserInput    bool
	groupByProgramInput bool
	Zone                string
	Address             string
}

var (
	processFilterFlagValues ProcessFilterFlagValues
)

func SetProcessFilterFlags(command *cobra.Command) {
	command.Flags().BoolVar(&processFilterFlagValues.groupByUserInput, "groupbyuser", false, "Group processes by user")
	command.Flags().BoolVar(&processFilterFlagValues.groupByProgramInput, "groupbyprog", false, "Group processes by client program")
	command.Flags().StringVar(&processFilterFlagValues.Zone, "zone", "", "Display processes from the specified zone")
	command.Flags().StringVar(&processFilterFlagValues.Address, "address", "", "Display processes from the specified address")
}

func GetProcessFilterFlagValues() *ProcessFilterFlagValues {
	if processFilterFlagValues.groupByUserInput {
		processFilterFlagValues.GroupBy = ProcessGroupByUser
	} else if processFilterFlagValues.groupByProgramInput {
		processFilterFlagValues.GroupBy = ProcessGroupByProgram
	} else {
		processFilterFlagValues.GroupBy = ProcessGroupByNone
	}

	return &processFilterFlagValues
}

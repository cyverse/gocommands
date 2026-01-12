package flag

import (
	"github.com/spf13/cobra"
)

type TouchFlagValues struct {
	NoCreate                 bool
	ReplicaNumber            int
	ReplicaNumberUpdated     bool
	ReferencePath            string
	SecondsSinceEpoch        int
	SecondsSinceEpochUpdated bool
}

var (
	touchFlagValues TouchFlagValues
)

func SetNoCreateFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&touchFlagValues.NoCreate, "no_create", "", false, "Skip creation of the data object")
	command.Flags().IntVarP(&touchFlagValues.ReplicaNumber, "replica", "n", 0, "The replica number of the replica to update. Cannot be used with -R")
	command.Flags().StringVarP(&touchFlagValues.ReferencePath, "reference", "r", "", "Use the modification time of the data object given instead of the current time. Cannot be used with -s")
	command.Flags().IntVarP(&touchFlagValues.SecondsSinceEpoch, "seconds-since-epoch", "", 0, "Use the modification time given in seconds since epoch instead of the current time. Cannot be used with -r")

	command.MarkFlagsMutuallyExclusive("reference", "replica")
	command.MarkFlagsMutuallyExclusive("reference", "seconds-since-epoch")
}

func GetTouchFlagValues(command *cobra.Command) *TouchFlagValues {
	if command.Flags().Changed("replica") {
		touchFlagValues.ReplicaNumberUpdated = true
	}

	if command.Flags().Changed("seconds-since-epoch") {
		touchFlagValues.SecondsSinceEpochUpdated = true
	}

	return &touchFlagValues
}

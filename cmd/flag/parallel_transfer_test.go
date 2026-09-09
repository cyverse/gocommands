package flag

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestGetParallelTransferFlagValuesCapsThreadsPerFile(t *testing.T) {
	original := parallelTransferFlagValues
	defer func() {
		parallelTransferFlagValues = original
	}()

	parallelTransferFlagValues = ParallelTransferFlagValues{
		ThreadNumber:        4,
		ThreadNumberPerFile: 16,
		tcpBufferSizeInput:  "0",
	}

	values, err := GetParallelTransferFlagValues()
	if err != nil {
		t.Fatalf("GetParallelTransferFlagValues() error = %v", err)
	}
	if values.ThreadNumberPerFile != 4 {
		t.Errorf("ThreadNumberPerFile = %d, want 4", values.ThreadNumberPerFile)
	}
}

func TestGetParallelTransferFlagValuesRejectsInvalidBufferSize(t *testing.T) {
	original := parallelTransferFlagValues
	defer func() {
		parallelTransferFlagValues = original
	}()

	parallelTransferFlagValues.tcpBufferSizeInput = ""
	if _, err := GetParallelTransferFlagValues(); err == nil {
		t.Fatal("GetParallelTransferFlagValues() error = nil, want invalid size error")
	}
}

func TestGetParallelTransferFlagValuesRejectsLargeBufferSize(t *testing.T) {
	original := parallelTransferFlagValues
	defer func() {
		parallelTransferFlagValues = original
	}()

	parallelTransferFlagValues.tcpBufferSizeInput = "101M"
	if _, err := GetParallelTransferFlagValues(); err == nil {
		t.Fatal("GetParallelTransferFlagValues() error = nil, want oversized buffer error")
	}
}

func TestSetLogLevelRejectsInvalidLevel(t *testing.T) {
	original := commonFlagValues
	defer func() {
		commonFlagValues = original
	}()

	command := &cobra.Command{}
	commonFlagValues.logLevelInput = "not-a-level"
	if err := setLogLevel(command); err == nil {
		t.Fatal("setLogLevel() error = nil, want invalid level error")
	}
}

func TestGetTicketFlagValuesRejectsInvalidType(t *testing.T) {
	original := ticketFlagValues
	defer func() {
		ticketFlagValues = original
	}()

	ticketFlagValues.typeInput = "writ"
	if _, err := GetTicketFlagValues(); err == nil {
		t.Fatal("GetTicketFlagValues() error = nil, want invalid ticket type error")
	}
}

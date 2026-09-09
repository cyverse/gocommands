package flag

import "testing"

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

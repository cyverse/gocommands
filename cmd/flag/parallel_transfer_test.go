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

	values := GetParallelTransferFlagValues()
	if values.ThreadNumberPerFile != 4 {
		t.Errorf("ThreadNumberPerFile = %d, want 4", values.ThreadNumberPerFile)
	}
}

package commons

import irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"

func CalculateThreadForTransferJob(fileSize int64, maxThreads int) int {
	if maxThreads <= 1 {
		return 1
	}

	threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(fileSize)
	if threadsRequired > maxThreads {
		return maxThreads
	}

	// if file is large enough to be prioritized, use max threads
	if fileSize >= minSizePrioritizedTransfer {
		factor := fileSize / minSizePrioritizedTransfer
		if factor >= 10 {
			threadsRequired = threadsRequired * 4 // max 16
		} else {
			threadsRequired = threadsRequired * 2 // 8
		}
	}

	if threadsRequired > maxThreads {
		return maxThreads
	}
	return threadsRequired
}

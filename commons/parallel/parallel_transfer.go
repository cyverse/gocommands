package parallel

import irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"

func CalculateThreadForTransferJob(fileSize int64, maxThreads int) int {
	if maxThreads <= 1 {
		return 1
	}

	threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(fileSize)
	if threadsRequired > maxThreads {
		return maxThreads
	}

	return threadsRequired
}

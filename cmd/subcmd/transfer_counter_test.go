package subcmd

import (
	"sync"
	"testing"
)

const concurrentTransfers = 100

func TestTransferCountersAreSafeForConcurrentUpdates(t *testing.T) {
	var get GetCommand
	var put PutCommand
	var bput BputCommand

	var wait sync.WaitGroup
	for range concurrentTransfers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			get.addDownloaded(10)
			put.addUploaded(1, 10)
			bput.addUploaded(2, 20)
		}()
	}
	wait.Wait()

	getFiles, getBytes := get.downloadedTotals()
	assertTotals(t, "get", getFiles, getBytes, concurrentTransfers, concurrentTransfers*10)
	putFiles, putBytes := put.uploadedTotals()
	assertTotals(t, "put", putFiles, putBytes, concurrentTransfers, concurrentTransfers*10)
	bputFiles, bputBytes := bput.uploadedTotals()
	assertTotals(t, "bput", bputFiles, bputBytes, concurrentTransfers*2, concurrentTransfers*20)
}

func assertTotals(t *testing.T, name string, files int64, bytes int64, wantFiles int, wantBytes int) {
	t.Helper()
	if files != int64(wantFiles) || bytes != int64(wantBytes) {
		t.Errorf("%s totals = (%d files, %d bytes), want (%d files, %d bytes)", name, files, bytes, wantFiles, wantBytes)
	}
}

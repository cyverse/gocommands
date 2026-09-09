package parallel

import (
	"testing"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
)

func TestParallelJobManagerRunsOversizedWeight(t *testing.T) {
	manager := NewParallelJobManager(2, false, false, false)
	run := make(chan int, 1)
	manager.Schedule("oversized", func(job *ParallelJob) error {
		run <- job.GetWeight()
		return nil
	}, 3, progress.UnitsDefault)

	done := make(chan error, 1)
	go func() {
		done <- manager.Start()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start() did not finish for a job whose requested weight exceeds capacity")
	}

	if weight := <-run; weight != 2 {
		t.Errorf("scheduled weight = %d, want 2", weight)
	}
}

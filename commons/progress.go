package commons

type ProgressTrackerCallback func(name string, processed int64, total int64)

type TransferProgress struct {
	name     string
	progress int64
	max      int64
}

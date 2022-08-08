package commons

type ProgressTrackerCallback func(name string, processed int64, total int64, done bool)

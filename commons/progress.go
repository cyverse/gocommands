package commons

import "github.com/jedib0t/go-pretty/v6/progress"

type ProgressTrackerCallback func(name string, processed int64, total int64, unit progress.Units, errored bool)

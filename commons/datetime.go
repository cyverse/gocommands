package commons

import (
	"time"
)

func MakeDateTimeString(t time.Time) string {
	return t.Format("2006-01-02.15:04")
}

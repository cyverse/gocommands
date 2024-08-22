package commons

import (
	"strings"
	"time"

	"golang.org/x/xerrors"
)

const (
	datetimeLayout string = "2006-01-02 15:04:05"
)

func MakeDateTimeString(t time.Time) string {
	return t.Format("2006-01-02.15:04")
}

func MakeDateTimeFromString(str string) (time.Time, error) {
	if len(str) == 0 || str == "0" {
		return time.Time{}, nil
	}

	if strings.HasPrefix(str, "+") {
		// duration
		dur, err := time.ParseDuration(str[1:])
		if err != nil {
			return time.Time{}, xerrors.Errorf("failed to parse duration: %w", err)
		}

		return time.Now().Add(dur), nil
	}

	t, err := time.Parse(datetimeLayout, str)
	if err != nil {
		return time.Time{}, xerrors.Errorf("failed to parse time %q: %w", str, err)
	}

	return t, nil
}

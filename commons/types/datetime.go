package types

import (
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	datetimeLayout1 string = "2006-01-02 15:04:05"
	datetimeLayout2 string = "2006:01:02 15:04:05"
)

func MakeDateTimeStringHM(t time.Time) string {
	return t.Format("2006-01-02.15:04")
}

func MakeDateTimeString(t time.Time) string {
	return t.Format("2006-01-02.15:04:05")
}

func MakeDateTimeFromString(str string) (time.Time, error) {
	if len(str) == 0 || str == "0" {
		return time.Time{}, nil
	}

	if strings.HasPrefix(str, "+") {
		// duration
		dur, err := time.ParseDuration(str[1:])
		if err != nil {
			return time.Time{}, errors.Wrapf(err, "failed to parse duration")
		}

		return time.Now().Add(dur), nil
	}

	t1, err1 := time.Parse(datetimeLayout1, str)
	if err1 != nil {
		// try second
		t2, err2 := time.Parse(datetimeLayout2, str)
		if err2 != nil {
			return time.Time{}, errors.Wrapf(err2, "failed to parse time %q", str)
		} else {
			return t2, nil
		}
	}

	return t1, nil
}

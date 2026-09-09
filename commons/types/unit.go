package types

import (
	"math"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

const (
	KiloBytes int64 = 1024
	MegaBytes int64 = KiloBytes * 1024
	GigaBytes int64 = MegaBytes * 1024
	TeraBytes int64 = GigaBytes * 1024

	Minute int = 60
	Hour   int = Minute * 60
	Day    int = Hour * 24
)

func ParseSize(size string) (int64, error) {
	size = strings.TrimSpace(size)
	size = strings.ToUpper(size)
	size = strings.TrimSuffix(size, "B")
	if len(size) == 0 {
		return 0, errors.New("size must not be empty")
	}

	sizeNum := int64(0)
	var err error

	multiplier := int64(1)
	switch size[len(size)-1] {
	case 'K', 'M', 'G', 'T':
		sizeNum, err = strconv.ParseInt(size[:len(size)-1], 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", size)
		}
		switch size[len(size)-1] {
		case 'K':
			multiplier = KiloBytes
		case 'M':
			multiplier = MegaBytes
		case 'G':
			multiplier = GigaBytes
		case 'T':
			multiplier = TeraBytes
		}
	default:
		sizeNum, err = strconv.ParseInt(size, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", size)
		}
		return sizeNum, nil
	}

	if sizeNum > math.MaxInt64/multiplier || sizeNum < math.MinInt64/multiplier {
		return 0, errors.Errorf("size %q is out of range", size)
	}
	return sizeNum * multiplier, nil
}

func SizeString(bytes int64) string {
	if bytes >= TeraBytes {
		return strconv.FormatFloat(float64(bytes)/float64(TeraBytes), 'f', 2, 64) + "TB"
	} else if bytes >= GigaBytes {
		return strconv.FormatFloat(float64(bytes)/float64(GigaBytes), 'f', 2, 64) + "GB"
	} else if bytes >= MegaBytes {
		return strconv.FormatFloat(float64(bytes)/float64(MegaBytes), 'f', 2, 64) + "MB"
	} else if bytes >= KiloBytes {
		return strconv.FormatFloat(float64(bytes)/float64(KiloBytes), 'f', 2, 64) + "KB"
	} else {
		return strconv.FormatInt(bytes, 10) + "B"
	}
}

func ParseTime(t string) (int, error) {
	t = strings.TrimSpace(t)
	t = strings.ToUpper(t)
	if len(t) == 0 {
		return 0, errors.New("time must not be empty")
	}

	tNum := int64(0)
	var err error

	multiplier := int64(1)
	switch t[len(t)-1] {
	case 'S', 'M', 'H', 'D':
		tNum, err = strconv.ParseInt(t[:len(t)-1], 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", t)
		}
		switch t[len(t)-1] {
		case 'M':
			multiplier = int64(Minute)
		case 'H':
			multiplier = int64(Hour)
		case 'D':
			multiplier = int64(Day)
		}
	default:
		tNum, err = strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", t)
		}
		multiplier = 1
	}

	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if tNum > maxInt/multiplier || tNum < minInt/multiplier {
		return 0, errors.Errorf("time %q is out of range", t)
	}
	return int(tNum * multiplier), nil
}

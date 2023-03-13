package commons

import (
	"strconv"
	"strings"

	"golang.org/x/xerrors"
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

	sizeNum := int64(0)
	var err error

	switch size[len(size)-1] {
	case 'K', 'M', 'G', 'T':
		sizeNum, err = strconv.ParseInt(size[:len(size)-1], 10, 64)
		if err != nil {
			return 0, xerrors.Errorf("failed to convert string '%s' to int: %w", size, err)
		}
	default:
		sizeNum, err = strconv.ParseInt(size, 10, 64)
		if err != nil {
			return 0, xerrors.Errorf("failed to convert string '%s' to int: %w", size, err)
		}
		return sizeNum, nil
	}

	switch size[len(size)-1] {
	case 'K':
		return sizeNum * KiloBytes, nil
	case 'M':
		return sizeNum * MegaBytes, nil
	case 'G':
		return sizeNum * GigaBytes, nil
	case 'T':
		return sizeNum * TeraBytes, nil
	default:
		return sizeNum, nil
	}
}

func ParseTime(t string) (int, error) {
	t = strings.TrimSpace(t)
	t = strings.ToUpper(t)

	tNum := int64(0)
	var err error

	switch t[len(t)-1] {
	case 'S', 'M', 'H', 'D':
		tNum, err = strconv.ParseInt(t[:len(t)-1], 10, 64)
		if err != nil {
			return 0, xerrors.Errorf("failed to convert string '%s' to int: %w", t, err)
		}
	default:
		tNum, err = strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, xerrors.Errorf("failed to convert string '%s' to int: %w", t, err)
		}
		return int(tNum), nil
	}

	switch t[len(t)-1] {
	case 'M':
		return int(tNum) * Minute, nil
	case 'H':
		return int(tNum) * Hour, nil
	case 'D':
		return int(tNum) * Day, nil
	default:
		return int(tNum), nil
	}
}

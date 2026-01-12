package path

import (
	"os"
	"strings"
)

func GetBasename(p string) string {
	p = strings.TrimRight(p, string(os.PathSeparator))
	p = strings.TrimRight(p, "/")

	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return p
	}

	if idx1 >= idx2 {
		return p[idx1+1:]
	}
	return p[idx2+1:]
}

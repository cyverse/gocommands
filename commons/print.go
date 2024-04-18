package commons

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func Println(a ...any) (n int, err error) {
	if log.GetLevel() > log.InfoLevel {
		return fmt.Println(a...)
	}
	return 0, nil
}

func Printf(format string, a ...any) (n int, err error) {
	if log.GetLevel() > log.InfoLevel {
		return fmt.Printf(format, a...)
	}
	return 0, nil
}

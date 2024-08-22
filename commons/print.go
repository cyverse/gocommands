package commons

import (
	"fmt"
	"io"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	terminalOutput *TerminalWriter
)

type TerminalWriter struct {
	mutex sync.Mutex
}

func (writer *TerminalWriter) Write(p []byte) (n int, err error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return os.Stdout.Write(p)
}

func (writer *TerminalWriter) Lock() {
	writer.mutex.Lock()
}

func (writer *TerminalWriter) Unlock() {
	writer.mutex.Unlock()
}

func InitTerminalOutput() {
	terminalOutput = &TerminalWriter{}
}

func GetTerminalWriter() *TerminalWriter {
	return terminalOutput
}

func PrintInfoln(a ...any) (n int, err error) {
	if log.GetLevel() > log.InfoLevel {
		return Println(a...)
	}
	return 0, nil
}

func PrintInfof(format string, a ...any) (n int, err error) {
	if log.GetLevel() > log.InfoLevel {
		return Printf(format, a...)
	}
	return 0, nil
}

func Print(a ...any) (n int, err error) {
	return fmt.Fprint(terminalOutput, a...)
}

func Printf(format string, a ...any) (n int, err error) {
	return fmt.Fprintf(terminalOutput, format, a...)
}

func Println(a ...any) (n int, err error) {
	return fmt.Fprintln(terminalOutput, a...)
}

func Fprintf(w io.Writer, format string, a ...any) (int, error) {
	terminalOutput.Lock()
	defer terminalOutput.Unlock()

	return fmt.Fprintf(w, format, a...)
}

func PrintErrorf(format string, a ...any) (int, error) {
	terminalOutput.Lock()
	defer terminalOutput.Unlock()

	red := "\033[31m"
	reset := "\033[0m"

	fmt.Fprint(os.Stderr, red)
	n, err := fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, reset)

	return n, err
}

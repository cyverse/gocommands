package terminal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"golang.org/x/term"
)

type ProgressTrackerCallback func(name string, processed int64, total int64, unit progress.Units, errored bool)

const (
	progressTrackerLength        int = 20
	progressMessageLengthMin     int = 20
	progressMessageLengthDefault int = 40
	progressTerminalWidthDefault int = 80
)

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		width = progressTerminalWidthDefault
	}

	return width
}

func GetProgressMessageWidth(displayPath bool) int {
	if displayPath {
		twidth := getTerminalWidth()

		messageWidth := twidth - progressTrackerLength - 50
		if messageWidth <= 0 {
			messageWidth = progressMessageLengthMin
		}

		return messageWidth
	} else {
		return progressMessageLengthDefault
	}
}

func GetProgressWriter(displayPath bool) progress.Writer {
	progressWriter := progress.NewWriter()
	progressWriter.SetOutputWriter(GetTerminalWriter())
	progressWriter.SetAutoStop(false)
	progressWriter.SetTrackerLength(progressTrackerLength)
	progressWriter.SetMessageLength(GetProgressMessageWidth(displayPath))
	progressWriter.SetStyle(progress.StyleDefault)
	progressWriter.SetTrackerPosition(progress.PositionRight)
	progressWriter.SetUpdateFrequency(time.Millisecond * 100)
	progressWriter.Style().Colors = progress.StyleColorsExample
	progressWriter.Style().Options.PercentFormat = "%4.1f%%"
	progressWriter.Style().Options.TimeInProgressPrecision = 100 * time.Millisecond
	progressWriter.Style().Options.TimeOverallPrecision = time.Second
	progressWriter.Style().Options.TimeDonePrecision = 10 * time.Millisecond
	progressWriter.Style().Visibility.ETA = true
	progressWriter.Style().Visibility.Percentage = true
	progressWriter.Style().Visibility.Time = true
	progressWriter.Style().Visibility.Value = true
	progressWriter.Style().Visibility.ETAOverall = false
	progressWriter.Style().Visibility.TrackerOverall = false

	return progressWriter
}

func GetShortPathMessage(name string, messageLen int) string {
	msg := name
	if messageLen < len(name) {
		shortname := name[len(name)-messageLen+4:]

		idx := firstPathSeparatorIndex(shortname)
		if idx > 0 {
			shortname = shortname[idx:]
		} else {
			shortname = fmt.Sprintf("/%s", getBasenameOfPath(name))
		}

		msg = fmt.Sprintf("...%s", shortname)
	}

	return msg
}

func firstPathSeparatorIndex(p string) int {
	idx1 := strings.Index(p, string(os.PathSeparator))
	idx2 := strings.Index(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return idx1
	}

	if idx1 < 0 {
		return idx2
	}

	if idx2 < 0 {
		return idx1
	}

	if idx1 <= idx2 {
		return idx1
	}

	return idx2
}

func getBasenameOfPath(p string) string {
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

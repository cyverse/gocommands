package commons

import (
	"fmt"
	"os"
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

func getProgressMessageWidth(displayPath bool) int {
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
	progressWriter.SetMessageWidth(getProgressMessageWidth(displayPath))
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

		idx := FirstDelimeterIndex(shortname)
		if idx > 0 {
			shortname = shortname[idx:]
		} else {
			shortname = fmt.Sprintf("/%s", GetBasename(name))
		}

		msg = fmt.Sprintf("...%s", shortname)
	}

	return msg
}

package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderHandlesRowsShorterThanHeader(t *testing.T) {
	formats := []OutputFormat{OutputFormatTable, OutputFormatTSV, OutputFormatCSV, OutputFormatLegacy}
	for _, outputFormat := range formats {
		t.Run(string(outputFormat), func(t *testing.T) {
			var output bytes.Buffer
			formatter := NewOutputFormatter(&output)
			table := formatter.NewTable("test")
			table.SetHeader([]string{"first", "second"})
			table.AppendRow([]interface{}{"value"})

			formatter.Render(outputFormat)
			if !strings.Contains(output.String(), "value") {
				t.Errorf("Render(%q) output = %q, want row value", outputFormat, output.String())
			}
		})
	}
}

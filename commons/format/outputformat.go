package format

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

type OutputFormat string

const (
	// format
	OutputFormatTable OutputFormat = "table"
	OutputFormatTSV   OutputFormat = "tsv"
	OutputFormatCSV   OutputFormat = "csv"
	OutputFormatJSON  OutputFormat = "json"
)

// GetOutputFormat returns OutputFormat from string
func GetOutputFormat(order string) OutputFormat {
	switch strings.ToLower(order) {
	case string(OutputFormatTable):
		return OutputFormatTable
	case string(OutputFormatTSV):
		return OutputFormatTSV
	case string(OutputFormatCSV):
		return OutputFormatCSV
	case string(OutputFormatJSON):
		return OutputFormatJSON
	default:
		return OutputFormatTable
	}
}

type OutputFormatter struct {
	Writer io.Writer
	Tables []OutputFormatterTable
}

type OutputFormatterTable struct {
	Title    string
	Header   []string
	WidthMax []int
	Rows     [][]interface{}
}

type OutputFormatterTableObject struct {
	Title string                   `json:"title"`
	Data  []map[string]interface{} `json:"data"`
}

func NewOutputFormatter(writer io.Writer) *OutputFormatter {
	return &OutputFormatter{
		Writer: writer,
		Tables: []OutputFormatterTable{},
	}
}

func (of *OutputFormatter) NewTable(title string) *OutputFormatterTable {
	table := OutputFormatterTable{
		Title:    title,
		Header:   []string{},
		WidthMax: []int{},
		Rows:     [][]interface{}{},
	}
	of.Tables = append(of.Tables, table)
	return &of.Tables[len(of.Tables)-1]
}

func (of *OutputFormatter) GetCurrentTable() *OutputFormatterTable {
	if len(of.Tables) == 0 {
		return nil
	}

	return &of.Tables[len(of.Tables)-1]
}

func (of *OutputFormatter) Render(format OutputFormat) {
	if format == OutputFormatJSON {
		of.RenderJSON()
		return
	}

	for _, ofTable := range of.Tables {
		// table writer
		tableWriter := table.NewWriter()
		tableWriter.SetOutputMirror(of.Writer)
		tableWriter.SetTitle(ofTable.Title)

		// remove empty column
		nonEmptyColIndices := make(map[int]bool)
		for header := range ofTable.Header {
			isEmpty := true
			for _, row := range ofTable.Rows {
				if row[header] != "" && row[header] != nil {
					isEmpty = false
					break
				}
			}
			if !isEmpty {
				nonEmptyColIndices[header] = true
			}
		}

		// header
		headerVal := make([]interface{}, 0, len(nonEmptyColIndices))
		for headerIdx, h := range ofTable.Header {
			if _, ok := nonEmptyColIndices[headerIdx]; !ok {
				continue
			}

			headerVal = append(headerVal, h)
		}
		tableWriter.AppendHeader(table.Row(headerVal), table.RowConfig{})

		// table size control
		columnConfigs := []table.ColumnConfig{}
		for configIdx, maxWidth := range ofTable.WidthMax {
			if _, ok := nonEmptyColIndices[configIdx]; !ok {
				continue
			}

			columnConfigs = append(columnConfigs, table.ColumnConfig{
				Number:   configIdx + 1,
				WidthMax: maxWidth,
			})
		}
		tableWriter.SetColumnConfigs(columnConfigs)

		// rows
		for _, rows := range ofTable.Rows {
			rowVal := make([]interface{}, 0, len(nonEmptyColIndices))
			for rowIdx, row := range rows {
				if _, ok := nonEmptyColIndices[rowIdx]; !ok {
					continue
				}

				rowVal = append(rowVal, row)
			}

			tableWriter.AppendRow(table.Row(rowVal), table.RowConfig{})
		}

		// render
		switch format {
		case OutputFormatCSV:
			tableWriter.RenderCSV()
		case OutputFormatTSV:
			tableWriter.RenderTSV()
		default:
			tableWriter.Render()
		}
	}
}

func (of *OutputFormatter) GetJSON(pretty bool) string {
	tableObjectArray := make([]OutputFormatterTableObject, 0)
	for _, ofTable := range of.Tables {
		tableObject := ofTable.ToObject()
		tableObjectArray = append(tableObjectArray, tableObject)
	}

	// marshal
	var jsonStrBytes []byte
	var err error

	if pretty {
		jsonStrBytes, err = json.MarshalIndent(tableObjectArray, "", "  ")
	} else {
		jsonStrBytes, err = json.Marshal(tableObjectArray)
	}

	if err != nil {
		return "{}"
	}
	return string(jsonStrBytes)
}

func (of *OutputFormatter) RenderJSON() {
	jsonStr := of.GetJSON(true)
	of.Writer.Write([]byte(jsonStr))
	of.Writer.Write([]byte("\n"))
}

func (oft *OutputFormatterTable) SetHeader(header []string) {
	oft.Header = header
}

func (oft *OutputFormatterTable) SetColumnWidthMax(widthMax []int) {
	oft.WidthMax = widthMax
}

func (oft *OutputFormatterTable) AppendRow(row []interface{}) {
	oft.Rows = append(oft.Rows, row)
}

func (oft *OutputFormatterTable) AppendRows(rows [][]interface{}) {
	oft.Rows = append(oft.Rows, rows...)
}

func (oft *OutputFormatterTable) ToObject() OutputFormatterTableObject {
	mapData := OutputFormatterTableObject{
		Title: oft.Title,
		Data:  make([]map[string]interface{}, 0, len(oft.Rows)),
	}

	// convert rows
	for _, row := range oft.Rows {
		rowMap := make(map[string]interface{})
		for idx, cell := range row {
			if idx < len(oft.Header) {
				rowMap[oft.Header[idx]] = cell
			}
		}
		mapData.Data = append(mapData.Data, rowMap)
	}

	return mapData
}

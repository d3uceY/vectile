package parser

import (
	"log/slog"
	"strings"

	"github.com/xuri/excelize/v2"
)

// SpreadsheetDoc is the parsed representation of an XLSX workbook. Text holds
// one "## Sheet: <name>" markdown section per sheet so ChunkMarkdown splits
// the workbook sheet by sheet.
type SpreadsheetDoc struct {
	Sheets []string
	Text   string
}

// maxSheetRows bounds the rows emitted for one sheet. Real workbooks never get
// close; the guard stops a pathological sheet from ballooning memory.
const maxSheetRows = 200000

// ParseXLSX extracts each sheet of an .xlsx workbook as a markdown table
// section: "## Sheet: <name>" followed by pipe-joined rows. Formula cells read
// as their last cached value.
func ParseXLSX(path string) *SpreadsheetDoc {
	doc := &SpreadsheetDoc{}
	f, err := excelize.OpenFile(path)
	if err != nil {
		slog.Error("failed to parse XLSX", "path", path, "err", err)
		return doc
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("closing xlsx workbook", "path", path, "err", err)
		}
	}()

	sheets := f.GetSheetList()
	doc.Sheets = sheets

	var parts []string
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			slog.Warn("cannot read xlsx sheet", "sheet", sheet, "err", err)
			continue
		}
		var lines []string
		for i, row := range rows {
			if i >= maxSheetRows {
				slog.Warn("xlsx sheet exceeds row cap, truncating", "sheet", sheet, "cap", maxSheetRows)
				break
			}
			if len(row) == 0 {
				continue
			}
			allEmpty := true
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				continue
			}
			lines = append(lines, strings.TrimSpace(strings.Join(row, " | ")))
		}
		if len(lines) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString("## Sheet: ")
		b.WriteString(sheet)
		b.WriteByte('\n')
		for _, line := range lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		parts = append(parts, b.String())
	}

	doc.Text = strings.Join(parts, "\n")
	return doc
}

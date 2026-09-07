package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestParseXLSX(t *testing.T) {
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "Revenue")
	_ = f.SetSheetRow("Revenue", "A1", &[]any{"Month", "Amount"})
	_ = f.SetSheetRow("Revenue", "A2", &[]any{"Jan", "100"})
	_ = f.SetSheetRow("Revenue", "A3", &[]any{"", ""}) // all-empty row, must be skipped
	_, _ = f.NewSheet("Notes")
	_ = f.SetCellValue("Notes", "A1", "hello world")

	p := filepath.Join(t.TempDir(), "book.xlsx")
	if err := f.SaveAs(p); err != nil {
		t.Fatalf("save workbook: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close workbook: %v", err)
	}

	doc := ParseXLSX(p)
	if len(doc.Sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %v", doc.Sheets)
	}
	for _, want := range []string{"## Sheet: Revenue", "Month | Amount", "Jan | 100", "## Sheet: Notes", "hello world"} {
		if !strings.Contains(doc.Text, want) {
			t.Fatalf("xlsx text missing %q:\n%s", want, doc.Text)
		}
	}
	if strings.Contains(doc.Text, "|  |") {
		t.Fatalf("all-empty row leaked into text:\n%s", doc.Text)
	}
}

func TestParseXLSXMissingFile(t *testing.T) {
	doc := ParseXLSX(filepath.Join(t.TempDir(), "nope.xlsx"))
	if doc.Text != "" {
		t.Fatalf("expected empty text for missing file, got %q", doc.Text)
	}
}

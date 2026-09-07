package parser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
)

// ParsePlaintext reads a plain text file and returns its content.
func ParsePlaintext(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read file", "path", path, "err", err)
		return ""
	}
	return string(data)
}

// ParseCSVText parses a CSV file and returns rows joined with " | " so table
// structure survives embedding and FTS. On a malformed/oversized CSV it falls
// back to the raw text rather than dropping the file.
func ParseCSVText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read CSV file", "path", path, "err", err)
		return ""
	}
	rows, err := readCSVRows(data)
	if err != nil {
		slog.Warn("CSV parse failed, indexing raw content", "path", path, "err", err)
		return string(data)
	}

	var b strings.Builder
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		line := strings.TrimSpace(strings.Join(row, " | "))
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func readCSVRows(data []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1 // tolerate ragged rows; keep all columns
	return r.ReadAll()
}

// ParseJSONText returns valid JSON pretty-printed (minified JSON is a single
// whitespace-less run, which defeats word-based chunking). Invalid JSON is
// returned raw rather than dropped.
func ParseJSONText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read JSON file", "path", path, "err", err)
		return ""
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		slog.Warn("JSON parse failed, indexing raw content", "path", path, "err", err)
		return string(data)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(pretty)
}


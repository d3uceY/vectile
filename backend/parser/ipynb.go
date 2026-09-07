package parser

import (
	"encoding/json"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// NotebookDoc is the parsed representation of a Jupyter .ipynb notebook.
// Text holds markdown cells verbatim and code cells as fenced blocks;
// outputs, execution counts, and base64 images are dropped.
type NotebookDoc struct {
	Kernel        string
	CodeCells     int
	MarkdownCells int
	Text          string
}

// notebookModel decodes only the fields the parser needs from an .ipynb.
type notebookModel struct {
	Metadata struct {
		Kernelspec struct {
			DisplayName string `json:"display_name"`
			Name        string `json:"name"`
		} `json:"kernelspec"`
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
	} `json:"metadata"`
	Cells []struct {
		CellType string          `json:"cell_type"`
		Source   json.RawMessage `json:"source"`
	} `json:"cells"`
}

var fenceLabelRE = regexp.MustCompile(`[^A-Za-z0-9_+.-]`)

// ParseNotebook extracts searchable text from a Jupyter notebook: markdown
// cells verbatim, code cells wrapped in a language fence. Everything else
// (outputs, base64, execution counts, cell metadata) is ignored. Malformed
// JSON is indexed as raw text rather than dropped, mirroring the CSV/JSON
// fallback behavior.
func ParseNotebook(path string) *NotebookDoc {
	doc := &NotebookDoc{}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read notebook", "path", path, "err", err)
		return doc
	}

	var nb notebookModel
	if err := json.Unmarshal(data, &nb); err != nil {
		slog.Warn("notebook is not valid JSON, indexing raw content", "path", path)
		doc.Text = strings.TrimSpace(string(data))
		return doc
	}

	kernel := nb.Metadata.Kernelspec.DisplayName
	if kernel == "" {
		kernel = nb.Metadata.Kernelspec.Name
	}
	if kernel == "" {
		kernel = "unknown"
	}
	doc.Kernel = kernel
	lang := nb.Metadata.LanguageInfo.Name
	if lang == "" {
		lang = "python"
	}
	lang = fenceLabelRE.ReplaceAllString(lang, "")

	var parts []string
	for _, cell := range nb.Cells {
		source, ok := cellSource(cell.Source)
		if !ok {
			continue
		}
		switch cell.CellType {
		case "markdown":
			if strings.TrimSpace(source) == "" {
				continue
			}
			parts = append(parts, source)
			doc.MarkdownCells++
		case "code":
			if strings.TrimSpace(source) == "" {
				continue
			}
			parts = append(parts, "```"+lang+"\n"+source+"\n```")
			doc.CodeCells++
		}
	}

	doc.Text = strings.Join(parts, "\n\n")
	return doc
}

// cellSource normalizes a notebook cell's "source" field, which per the
// nbformat spec is a string or a list of strings. Anything else (numbers,
// dicts, mixed lists) is treated as absent so a malformed-but-parseable
// notebook degrades gracefully instead of panicking.
func cellSource(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	var lines []string
	if err := json.Unmarshal(raw, &lines); err == nil {
		return strings.Join(lines, ""), true
	}
	var anyLines []any
	if err := json.Unmarshal(raw, &anyLines); err == nil {
		var b strings.Builder
		for _, v := range anyLines {
			if str, ok := v.(string); ok {
				b.WriteString(str)
			}
		}
		return b.String(), true
	}
	return "", false
}

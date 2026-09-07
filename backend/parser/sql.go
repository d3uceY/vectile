package parser

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// SQLDoc is the parsed representation of a SQL file. Text is the raw content;
// Tables and Statements are de-duplicated structural lists.
type SQLDoc struct {
	Tables     []string
	Statements []string
	Text       string
}

// sqlTableRe matches table names after table-introducing keywords
// (CREATE/ALTER TABLE, FROM, JOIN, INTO), with an optional IF [NOT] EXISTS.
var sqlTableRe = regexp.MustCompile(`(?i)\b(?:CREATE\s+TABLE|ALTER\s+TABLE|FROM|JOIN|INTO)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)`)

// sqlStmtRe matches the statement kinds present in a file.
var sqlStmtRe = regexp.MustCompile(`(?i)\b(CREATE|ALTER|DROP|SELECT|INSERT|UPDATE|DELETE)\b`)

// ParseSQLFile reads a SQL file, extracting a de-duplicated list of table
// names and statement kinds.
func ParseSQLFile(path string) *SQLDoc {
	doc := &SQLDoc{}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read SQL file", "path", path, "err", err)
		return doc
	}
	content := string(data)
	doc.Text = strings.TrimSpace(content)

	seen := map[string]bool{}
	for _, m := range sqlTableRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			doc.Tables = append(doc.Tables, name)
		}
		if len(doc.Tables) >= 50 {
			break
		}
	}

	stmtSeen := map[string]bool{}
	for _, m := range sqlStmtRe.FindAllStringSubmatch(content, -1) {
		k := strings.ToUpper(m[1])
		if !stmtSeen[k] {
			stmtSeen[k] = true
			doc.Statements = append(doc.Statements, k)
		}
	}
	return doc
}

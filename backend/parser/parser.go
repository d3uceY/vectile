// Package parser extracts plain text from the file formats vectile indexes.
package parser

import (
	"path/filepath"
	"strings"
)

// ExtensionMap maps file extensions to source type names.
var ExtensionMap = map[string]string{
	".md":    "markdown",
	".pdf":   "pdf",
	".docx":  "docx",
	".dotx":  "docx",
	".html":  "html",
	".htm":   "html",
	".txt":   "plaintext",
	".csv":   "plaintext",
	".json":  "plaintext",
	".yaml":  "plaintext",
	".yml":   "plaintext",
	".epub":  "epub",
	".xlsx":  "xlsx",
	".pptx":  "pptx",
	".ipynb": "ipynb",
	".xml":   "xml",
	".sql":   "sql",
	".sh":    "shell",
	".bash":  "shell",
}

// SourceTypeForPath returns the source type for a file path by extension,
// or "" if the extension is not recognized.
func SourceTypeForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	return ExtensionMap[ext]
}

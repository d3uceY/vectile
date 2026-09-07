package parser

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// ShellDoc is the parsed representation of a shell script. Text is the raw
// content; Functions is the list of defined function names.
type ShellDoc struct {
	Functions []string
	Text      string
}

// shellFuncRe matches "name()" and "function name()" definitions.
var shellFuncRe = regexp.MustCompile(`(?m)^(?:function\s+)?([A-Za-z_]\w*)\s*\(\s*\)`)

// ParseShellFile reads a shell script, extracting defined function names.
func ParseShellFile(path string) *ShellDoc {
	doc := &ShellDoc{}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read shell file", "path", path, "err", err)
		return doc
	}
	doc.Text = strings.TrimSpace(string(data))

	seen := map[string]bool{}
	for _, m := range shellFuncRe.FindAllStringSubmatch(doc.Text, -1) {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			doc.Functions = append(doc.Functions, name)
		}
	}
	return doc
}

package parser

import (
	"strings"
	"testing"
)

const notebookJSON = `{
  "nbformat": 4,
  "metadata": {
    "kernelspec": {"display_name": "Python 3", "name": "python3"},
    "language_info": {"name": "python"}
  },
  "cells": [
    {"cell_type": "markdown", "source": ["# Title\n", "Intro text."]},
    {"cell_type": "code", "source": ["print('hi')\n", "# a comment"], "outputs": [{"output_type": "stream", "text": "hi"}], "execution_count": 1}
  ]
}`

func TestParseNotebook(t *testing.T) {
	doc := ParseNotebook(writeTempFile(t, "nb.ipynb", notebookJSON))
	if doc.Kernel != "Python 3" {
		t.Fatalf("kernel not captured: %q", doc.Kernel)
	}
	if doc.CodeCells != 1 || doc.MarkdownCells != 1 {
		t.Fatalf("cell counts wrong: code=%d markdown=%d", doc.CodeCells, doc.MarkdownCells)
	}
	for _, want := range []string{"# Title", "Intro text.", "```python", "print('hi')", "# a comment", "```"} {
		if !strings.Contains(doc.Text, want) {
			t.Fatalf("notebook text missing %q:\n%s", want, doc.Text)
		}
	}
	// Outputs and execution metadata must not be indexed.
	for _, banned := range []string{"output_type", "execution_count", "outputs"} {
		if strings.Contains(doc.Text, banned) {
			t.Fatalf("notebook text leaked %q:\n%s", banned, doc.Text)
		}
	}
}

func TestParseNotebookMalformed(t *testing.T) {
	// Invalid JSON falls back to raw content.
	doc := ParseNotebook(writeTempFile(t, "broken.ipynb", `{"cells": [`))
	if !strings.Contains(doc.Text, "cells") {
		t.Fatalf("malformed notebook should index raw text, got %q", doc.Text)
	}

	// A non-dict top level (valid JSON, not a notebook) also falls back raw.
	doc = ParseNotebook(writeTempFile(t, "list.ipynb", `[1, 2, 3]`))
	if !strings.Contains(doc.Text, "1, 2, 3") {
		t.Fatalf("non-dict notebook should index raw text, got %q", doc.Text)
	}
}

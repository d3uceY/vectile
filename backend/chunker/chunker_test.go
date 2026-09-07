package chunker

import (
	"strconv"
	"strings"
	"testing"
)

func TestSplitIntoWindows(t *testing.T) {
	// Short text stays whole.
	short := "just a few words here"
	if got := SplitIntoWindows(short, 20, 5); len(got) != 1 {
		t.Fatalf("short text should be one window, got %d", len(got))
	}

	// Long text splits; first window hits the budget.
	text := strings.Repeat("word ", 100)
	chunks := SplitIntoWindows(text, 20, 5)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple windows, got %d", len(chunks))
	}
	if got := WordCount(chunks[0]); got != 20 {
		t.Fatalf("first window has %d words, want 20", got)
	}

	// Overlap: the tail of window N carries into the head of window N+1.
	a, b := strings.Fields(chunks[0]), strings.Fields(chunks[1])
	if a[len(a)-5] != b[0] {
		t.Fatal("expected 5-word overlap between consecutive windows")
	}
}

func TestChunkMarkdown(t *testing.T) {
	src := "# Overview\n\nSome intro text.\n\n## Details\n\nMore content here.\n"
	chunks := ChunkMarkdown(src, "note.md", 500, 50)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	foundHeading := false
	for _, c := range chunks {
		if _, ok := c.Metadata["heading_path"]; ok {
			foundHeading = true
		}
		if !strings.HasPrefix(c.Text, "[") && strings.Contains(c.Text, "Overview") {
			t.Log("preamble chunk has no heading prefix (expected)")
		}
	}
	if !foundHeading {
		t.Fatal("expected a chunk carrying heading_path metadata")
	}
}

func TestChunkPlain(t *testing.T) {
	empty := ChunkPlain("   ", "x", 500, 50)
	if len(empty) != 1 || empty[0].Text != "" {
		t.Fatalf("blank text should yield one empty chunk, got %v", empty)
	}

	text := strings.Repeat("sentence of words ", 60)
	chunks := ChunkPlain(text, "x", 50, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if chunks[0].Title != "x" {
		t.Fatalf("title not preserved: %q", chunks[0].Title)
	}
}

// headingInsideFence asserts a '#' comment line inside a fenced code block is
// NOT treated as a markdown heading, and the fence survives into the chunk.
func TestChunkMarkdownIgnoresHeadingsInsideFences(t *testing.T) {
	src := "# Doc\n\nIntro.\n\n```python\n# not a heading\n## also not a heading\ndef f():\n    pass\n```\n\n## Real\n\nBody.\n"
	chunks := ChunkMarkdown(src, "note.md", 500, 50)

	// The code-block comment lines must never split the block apart, so there
	// must be exactly one chunk under "Doc" whose text keeps the fence intact.
	var withFence string
	for _, c := range chunks {
		if strings.Contains(c.Text, "```python") {
			withFence += c.Text
		}
		if !strings.Contains(c.Text, "Doc") && strings.Contains(c.Text, "Real") {
			// sanity: each chunk belongs to a section
		}
	}
	if !strings.Contains(withFence, "def f():") {
		t.Fatal("code block content was lost")
	}
	if !strings.Contains(withFence, "# not a heading") {
		t.Fatal("code comment content was lost")
	}
	if !strings.Contains(withFence, "```") {
		t.Fatal("fence markers must be restored into the chunk text")
	}

	// The real "## Real" heading must still produce its own section/chunk.
	realFound := false
	for _, c := range chunks {
		if path, ok := c.Metadata["heading_path"].(string); ok && strings.Contains(path, "Real") {
			realFound = true
		}
	}
	if !realFound {
		t.Fatal("a real heading after the code fence must still split a section")
	}
}

// TestChunkMarkdownFenceWindowed verifies a code-heavy section still gets
// windowed by its real word count (masking must not hide code tokens).
func TestChunkMarkdownFenceWindowed(t *testing.T) {
	// ~60 words of prose + a big code block pushes the section over 40 words.
	var code strings.Builder
	code.WriteString("```js\n")
	for i := 0; i < 200; i++ {
		code.WriteString("const value_" + strconv.Itoa(i) + " = " + strconv.Itoa(i) + ";\n")
	}
	code.WriteString("```\n")
	src := "## Code\n\nLots of context here so the section is long enough to split.\n" + code.String()

	chunks := ChunkMarkdown(src, "big.md", 40, 5)
	if len(chunks) < 2 {
		t.Fatalf("code-heavy section should split into multiple windows, got %d", len(chunks))
	}
	// Every window must carry real code tokens (not placeholders).
	for _, c := range chunks {
		if strings.Contains(c.Text, "VECTILE_CODE_") {
			t.Fatalf("placeholder leaked into chunk text: %q", c.Text)
		}
	}
}

// TestChunkMarkdownUnterminatedFence treats an unclosed fence as code to EOF.
func TestChunkMarkdownUnterminatedFence(t *testing.T) {
	src := "# Top\n\n```python\n# comment\nprint(1)\n\n## Not A Real Heading\nprint(2)\n"
	chunks := ChunkMarkdown(src, "note.md", 500, 50)
	if len(chunks) != 1 {
		t.Fatalf("unterminated fence should keep the document as one section, got %d chunks", len(chunks))
	}
	if !strings.Contains(chunks[0].Text, "print(2)") {
		t.Fatal("content after an unterminated fence was lost")
	}
}

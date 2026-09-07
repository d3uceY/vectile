// Package chunker provides text chunking strategies for different content
// types. All chunkers use word-based windows sized by the config.
package chunker

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Chunk represents a piece of text ready for embedding, with metadata.
type Chunk struct {
	Text       string
	Title      string
	Metadata   map[string]any
	ChunkIndex int
}

// WordCount estimates token count by splitting on whitespace.
func WordCount(text string) int {
	return len(strings.Fields(text))
}

// SplitIntoWindows splits text into overlapping word-based windows.
func SplitIntoWindows(text string, chunkSize, overlap int) []string {
	// Defensive: the settings UI clamps these, but a hand-edited config.json
	// can still slip through. chunkSize < 1 never advances the window start
	// (infinite loop) and overlap >= chunkSize walks backwards into a
	// negative slice bound, which panics.
	if chunkSize < 1 {
		chunkSize = 1
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize - 1
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if len(words) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(words) {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[start:end], " "))
		if end >= len(words) {
			break
		}
		start = end - overlap
	}
	return chunks
}

var headingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

func codePlaceholder(i int) string {
	return "<<VECTILE_CODE_" + strconv.Itoa(i) + ">>"
}

// codeFenceStart reports whether a line opens a fenced code block (``` or ~~~).
func codeFenceStart(line string) bool {
	if len(line) < 3 {
		return false
	}
	c := line[0]
	if c != '`' && c != '~' {
		return false
	}
	n := 0
	for n < len(line) && line[n] == c {
		n++
	}
	return n >= 3
}

// codeFenceClose reports whether a line closes a fence opened with char c.
func codeFenceClose(line string, c byte) bool {
	n := 0
	for n < len(line) && line[n] == c {
		n++
	}
	return n >= 3
}

// maskFencedBlocks replaces fenced code blocks with placeholder tokens so
// heading detection never fires on '#' lines inside code. The blocks are
// returned in order for restoration. An unterminated fence runs to the end
// of the text.
func maskFencedBlocks(text string) (masked string, blocks []string) {
	var out strings.Builder
	var fence strings.Builder
	open := byte(0)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if open == 0 {
			if codeFenceStart(trimmed) {
				open = trimmed[0]
				fence.WriteString(line)
				fence.WriteByte('\n')
				continue
			}
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		fence.WriteString(line)
		fence.WriteByte('\n')
		if codeFenceClose(trimmed, open) {
			flushCodeFence(&out, &fence, &blocks)
			open = 0
		}
	}
	if open != 0 {
		flushCodeFence(&out, &fence, &blocks)
	}
	return out.String(), blocks
}

func flushCodeFence(out *strings.Builder, fence *strings.Builder, blocks *[]string) {
	if fence.Len() == 0 {
		return
	}
	idx := len(*blocks)
	*blocks = append(*blocks, fence.String())
	out.WriteString(codePlaceholder(idx))
	out.WriteByte('\n')
	fence.Reset()
}

// restoreCodeFences swaps placeholder tokens back for their original blocks.
func restoreCodeFences(masked string, blocks []string) string {
	for i, b := range blocks {
		masked = strings.ReplaceAll(masked, codePlaceholder(i), b)
	}
	return masked
}

// ChunkMarkdown splits markdown text on headings, preserving the heading path
// as a context prefix. Sections are chunked word-wise if they exceed the size.
//
// Fenced code blocks (``` / ~~~) are masked before the heading scan so '#'
// lines inside code never split sections, then restored into each section
// before word-counting so chunk sizes reflect the real content.
func ChunkMarkdown(text, title string, chunkSize, overlap int) []Chunk {
	if strings.TrimSpace(text) == "" {
		return []Chunk{{Text: "", Title: title, Metadata: map[string]any{}, ChunkIndex: 0}}
	}

	masked, codeBlocks := maskFencedBlocks(text)

	type section struct {
		headingPath string
		content     string
	}

	matches := headingPattern.FindAllStringSubmatchIndex(masked, -1)

	var sections []section
	if len(matches) == 0 {
		content := restoreCodeFences(strings.TrimSpace(masked), codeBlocks)
		sections = append(sections, section{headingPath: "", content: content})
	} else {
		preamble := restoreCodeFences(strings.TrimSpace(masked[:matches[0][0]]), codeBlocks)
		if preamble != "" {
			sections = append(sections, section{headingPath: "", content: preamble})
		}

		currentHeadings := make(map[int]string)
		for i, match := range matches {
			level := match[3] - match[2]
			headingText := strings.TrimSpace(masked[match[4]:match[5]])

			contentStart := match[1]
			contentEnd := len(masked)
			if i+1 < len(matches) {
				contentEnd = matches[i+1][0]
			}
			content := restoreCodeFences(strings.TrimSpace(masked[contentStart:contentEnd]), codeBlocks)

			currentHeadings[level] = headingText
			for k := range currentHeadings {
				if k > level {
					delete(currentHeadings, k)
				}
			}

			var levels []int
			for k := range currentHeadings {
				levels = append(levels, k)
			}
			sort.Ints(levels)
			var parts []string
			for _, l := range levels {
				parts = append(parts, currentHeadings[l])
			}
			headingPath := strings.Join(parts, " > ")

			if content != "" {
				sections = append(sections, section{headingPath: headingPath, content: content})
			}
		}
	}

	var chunks []Chunk
	chunkIdx := 0
	for _, sec := range sections {
		prefix := ""
		if sec.headingPath != "" {
			prefix = "[" + sec.headingPath + "] "
		}
		prefixed := prefix + sec.content

		if WordCount(prefixed) <= chunkSize {
			meta := map[string]any{}
			if sec.headingPath != "" {
				meta["heading_path"] = sec.headingPath
			}
			chunks = append(chunks, Chunk{Text: prefixed, Title: title, Metadata: meta, ChunkIndex: chunkIdx})
			chunkIdx++
		} else {
			prefixWords := WordCount(prefix)
			windows := SplitIntoWindows(sec.content, chunkSize-prefixWords, overlap)
			for _, w := range windows {
				meta := map[string]any{}
				if sec.headingPath != "" {
					meta["heading_path"] = sec.headingPath
				}
				chunks = append(chunks, Chunk{Text: prefix + w, Title: title, Metadata: meta, ChunkIndex: chunkIdx})
				chunkIdx++
			}
		}
	}

	if len(chunks) == 0 {
		chunks = append(chunks, Chunk{Text: strings.TrimSpace(text), Title: title, Metadata: map[string]any{}, ChunkIndex: 0})
	}
	return chunks
}

// ChunkPlain chunks plain text using fixed-size word windows with overlap.
func ChunkPlain(text, title string, chunkSize, overlap int) []Chunk {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []Chunk{{Text: "", Title: title, Metadata: map[string]any{}, ChunkIndex: 0}}
	}

	windows := SplitIntoWindows(trimmed, chunkSize, overlap)
	chunks := make([]Chunk, len(windows))
	for i, w := range windows {
		chunks[i] = Chunk{Text: w, Title: title, Metadata: map[string]any{}, ChunkIndex: i}
	}
	return chunks
}

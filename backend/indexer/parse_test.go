package indexer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"vectile/backend/chunker"
	"vectile/backend/config"
)

// parseCfg supplies only the chunk bounds parseAndChunk reads.
func parseCfg() *config.Config {
	return &config.Config{ChunkSizeTokens: 200, ChunkOverlapTokens: 40}
}

func joinChunks(chunks []chunker.Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString(c.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func writeParseFixture(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestParseAndChunkNewFormats(t *testing.T) {
	cfg := parseCfg()

	cases := []struct {
		name       string
		sourceType string
		path       string
		want       []string
	}{
		{"csv", "plaintext", writeParseFixture(t, "t.csv", "a,b\n1,2\n"), []string{"a | b", "1 | 2"}},
		{"json", "plaintext", writeParseFixture(t, "t.json", `{"x":1}`), []string{"\"x\": 1"}},
		{"xml", "xml", writeParseFixture(t, "t.xml", "<root><a>Tove</a></root>"), []string{"Tove"}},
		{"sql", "sql", writeParseFixture(t, "t.sql", "CREATE TABLE users (id INT);"), []string{"CREATE TABLE users"}},
		{"shell", "shell", writeParseFixture(t, "t.sh", "#!/bin/sh\nfoo() {\n echo hi\n}\n"), []string{"foo()"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := parseAndChunk(tc.path, tc.sourceType, cfg)
			if len(chunks) == 0 {
				t.Fatal("expected chunks, got none")
			}
			for _, want := range tc.want {
				if !strings.Contains(joinChunks(chunks), want) {
					t.Fatalf("output missing %q:\n%s", want, joinChunks(chunks))
				}
			}
		})
	}
}

func TestParseAndChunkXLSX(t *testing.T) {
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "Budget")
	_ = f.SetSheetRow("Budget", "A1", &[]any{"Item", "Cost"})
	_ = f.SetSheetRow("Budget", "A2", &[]any{"Laptops", "12000"})
	p := filepath.Join(t.TempDir(), "t.xlsx")
	if err := f.SaveAs(p); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}
	_ = f.Close()

	chunks := parseAndChunk(p, "xlsx", parseCfg())
	if len(chunks) == 0 {
		t.Fatal("expected xlsx chunks, got none")
	}
	text := joinChunks(chunks)
	for _, want := range []string{"[Sheet: Budget]", "Item | Cost", "Laptops | 12000"} {
		if !strings.Contains(text, want) {
			t.Fatalf("xlsx output missing %q:\n%s", want, text)
		}
	}
}

func TestParseAndChunkPPTX(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.pptx")
	w, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	const aml = "http://schemas.openxmlformats.org/drawingml/2006/main"
	const pml = "http://schemas.openxmlformats.org/presentationml/2006/main"
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	const slideRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	add := func(name, content string) {
		ef, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ef.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	add("ppt/presentation.xml",
		`<p:presentation xmlns:p="`+pml+`" xmlns:r="`+rel+`"><p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst></p:presentation>`)
	add("ppt/_rels/presentation.xml.rels",
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId2" Type="`+slideRel+`" Target="slides/slide1.xml"/></Relationships>`)
	add("ppt/slides/slide1.xml",
		`<p:sld xmlns:a="`+aml+`" xmlns:p="`+pml+`"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Quarterly review</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	chunks := parseAndChunk(p, "pptx", parseCfg())
	if len(chunks) == 0 {
		t.Fatal("expected pptx chunks, got none")
	}
	text := joinChunks(chunks)
	for _, want := range []string{"[Slide 1]", "Quarterly review"} {
		if !strings.Contains(text, want) {
			t.Fatalf("pptx output missing %q:\n%s", want, text)
		}
	}
}

func TestParseAndChunkNotebook(t *testing.T) {
	p := writeParseFixture(t, "t.ipynb",
		`{"cells":[{"cell_type":"markdown","source":["# Notes\n"]},{"cell_type":"code","source":["print('hello')"]}]}`)
	chunks := parseAndChunk(p, "ipynb", parseCfg())
	if len(chunks) == 0 {
		t.Fatal("expected notebook chunks, got none")
	}
	text := joinChunks(chunks)
	for _, want := range []string{"[Notes]", "print('hello')"} {
		if !strings.Contains(text, want) {
			t.Fatalf("notebook output missing %q:\n%s", want, text)
		}
	}
}

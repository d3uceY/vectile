package parser

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePPTXFixture builds a minimal PPTX zip: two slides whose zip names are
// deliberately out of numeric order (slide9 before slide2) so the test proves
// ordering comes from presentation.xml's sldIdLst, not filenames.
func writePPTXFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "deck.pptx")
	w, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)

	const pml = "http://schemas.openxmlformats.org/presentationml/2006/main"
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	const aml = "http://schemas.openxmlformats.org/drawingml/2006/main"
	const slideRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"

	add := func(name, content string) {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	add("ppt/presentation.xml",
		`<p:presentation xmlns:p="`+pml+`" xmlns:r="`+rel+`">`+
			`<p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId3"/></p:sldIdLst>`+
			`</p:presentation>`)
	add("ppt/_rels/presentation.xml.rels",
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId2" Type="`+slideRel+`" Target="slides/slide9.xml"/>`+
			`<Relationship Id="rId3" Type="`+slideRel+`" Target="slides/slide2.xml"/>`+
			`</Relationships>`)

	slide := func(text string) string {
		return `<p:sld xmlns:a="` + aml + `" xmlns:p="` + pml + `">` +
			`<p:cSld><p:spTree><p:sp><p:txBody><a:bodyPr/><a:lstStyle/>` +
			text +
			`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
	}
	para := func(text string) string {
		return `<a:p><a:r><a:t>` + text + `</a:t></a:r></a:p>`
	}

	add("ppt/slides/slide9.xml", slide(para("Alpha slide")+para("Beta line")))
	add("ppt/slides/slide2.xml", slide(para("Gamma slide")))

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParsePPTX(t *testing.T) {
	doc := ParsePPTX(writePPTXFixture(t))
	if doc.Slides != 2 {
		t.Fatalf("expected 2 slides, got %d", doc.Slides)
	}
	for _, want := range []string{"## Slide 1", "Alpha slide", "Beta line", "## Slide 2", "Gamma slide"} {
		if !strings.Contains(doc.Text, want) {
			t.Fatalf("pptx text missing %q:\n%s", want, doc.Text)
		}
	}
	// Order must follow sldIdLst (slide9's text first), not zip filename order.
	if strings.Index(doc.Text, "Alpha slide") > strings.Index(doc.Text, "Gamma slide") {
		t.Fatalf("slide order not preserved from sldIdLst:\n%s", doc.Text)
	}
}

func TestParsePPTXMissingFile(t *testing.T) {
	doc := ParsePPTX(filepath.Join(t.TempDir(), "nope.pptx"))
	if doc.Text != "" {
		t.Fatalf("expected empty text for missing file, got %q", doc.Text)
	}
}

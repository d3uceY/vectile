package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
)

// SlidesDoc is the parsed representation of a PPTX deck. Text holds one
// "## Slide N" markdown section per non-empty slide, in presentation order.
type SlidesDoc struct {
	Slides int
	Text   string
}

const slideRelType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"

const drawingMLMain = "http://schemas.openxmlformats.org/drawingml/2006/main"

// relsDoc models a .rels relationship part.
type pptxRels struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

// presentationModel keeps only the ordered slide references from
// ppt/presentation.xml; everything else is ignored.
type presentationModel struct {
	SldIdLst struct {
		SldIds []struct {
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sldId"`
	} `xml:"sldIdLst"`
}

// ParsePPTX extracts slide text from a PPTX file, in presentation order.
func ParsePPTX(path string) *SlidesDoc {
	doc := &SlidesDoc{}
	text, err := extractPPTXText(path)
	if err != nil {
		slog.Error("failed to parse PPTX", "path", path, "err", err)
		return doc
	}
	doc.Text = text
	doc.Slides = strings.Count(text, "## Slide ")
	return doc
}

func extractPPTXText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	targets, err := slideOrder(zr)
	if err != nil {
		slog.Warn("cannot resolve presentation slide order, assuming numeric slide files", "err", err)
		targets = numericSlides(zr)
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("no slides found in archive")
	}

	var parts []string
	for _, target := range targets {
		rc, err := zr.Open(target)
		if err != nil {
			slog.Warn("cannot open slide part, skipping", "part", target, "err", err)
			continue
		}
		slideText := parseSlideXML(rc)
		rc.Close()
		slideText = collapseBlankLines(slideText)
		if strings.TrimSpace(slideText) == "" {
			continue
		}
		parts = append(parts, "## Slide "+strconv.Itoa(len(parts)+1)+"\n"+slideText)
	}
	return strings.Join(parts, "\n\n"), nil
}

// slideOrder resolves the ordered list of slide zip entries from
// ppt/presentation.xml's sldIdLst plus ppt/_rels/presentation.xml.rels.
func slideOrder(zr *zip.Reader) ([]string, error) {
	relRC, err := zr.Open("ppt/_rels/presentation.xml.rels")
	if err != nil {
		return nil, err
	}
	defer relRC.Close()
	var rels pptxRels
	if err := xml.NewDecoder(relRC).Decode(&rels); err != nil {
		return nil, fmt.Errorf("parse presentation rels: %w", err)
	}

	byID := make(map[string]string)
	for _, rel := range rels.Relationships {
		if rel.Type == slideRelType && rel.Target != "" {
			byID[rel.ID] = normalizePartPath("ppt/", rel.Target)
		}
	}

	presRC, err := zr.Open("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	defer presRC.Close()
	var pres presentationModel
	if err := xml.NewDecoder(presRC).Decode(&pres); err != nil {
		return nil, fmt.Errorf("parse presentation.xml: %w", err)
	}

	var targets []string
	for _, sld := range pres.SldIdLst.SldIds {
		if t, ok := byID[sld.RID]; ok {
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no slide references resolved")
	}
	return targets, nil
}

// normalizePartPath joins a rel target under baseDir, rejecting traversal.
func normalizePartPath(baseDir, target string) string {
	target = strings.TrimPrefix(target, "/")
	target = strings.TrimPrefix(target, "./")
	if strings.Contains(target, "..") {
		return ""
	}
	return baseDir + target
}

// numericSlides is the fallback ordering: ppt/slides/slideN.xml by N.
func numericSlides(zr *zip.Reader) []string {
	type slide struct {
		n    int
		name string
	}
	var slides []slide
	for _, zf := range zr.File {
		var n int
		if _, err := fmt.Sscanf(zf.Name, "ppt/slides/slide%d.xml", &n); err == nil {
			slides = append(slides, slide{n: n, name: zf.Name})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].n < slides[j].n })
	var out []string
	for _, s := range slides {
		out = append(out, s.name)
	}
	return out
}

// parseSlideXML walks a slide part and collects <a:t> text runs, breaking
// lines at <a:p> paragraph and <a:br> break boundaries.
func parseSlideXML(r io.Reader) string {
	decoder := xml.NewDecoder(r)
	var buf strings.Builder
	inText := false
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("error reading slide xml", "err", err)
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == drawingMLMain && t.Name.Local == "t":
				inText = true
			case t.Name.Space == drawingMLMain && t.Name.Local == "br":
				buf.WriteByte('\n')
			case t.Name.Space == drawingMLMain && t.Name.Local == "tab":
				buf.WriteByte(' ')
			}
		case xml.CharData:
			if inText {
				buf.Write(t)
			}
		case xml.EndElement:
			if t.Name.Space == drawingMLMain && t.Name.Local == "t" {
				inText = false
			} else if t.Name.Space == drawingMLMain && t.Name.Local == "p" {
				buf.WriteByte('\n')
			}
		}
	}
	return buf.String()
}

// collapseBlankLines trims each line and squeezes 3+ newlines down to one
// blank separator line.
func collapseBlankLines(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	blank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if blank {
				continue
			}
			blank = true
			continue
		}
		if blank && len(out) > 0 {
			out = append(out, "")
		}
		blank = false
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

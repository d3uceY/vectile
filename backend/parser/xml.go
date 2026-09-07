package parser

import (
	"bytes"
	"encoding/xml"
	"io"
	"log/slog"
	"os"
	"strings"
)

// XMLDoc is the parsed representation of an XML file. Text is the raw content
// (mirrors the reference, which indexes XML as-is after recording structure).
type XMLDoc struct {
	RootElement string
	Text        string
}

// ParseXMLFile reads an XML file, recording its root element name.
func ParseXMLFile(path string) *XMLDoc {
	doc := &XMLDoc{}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read XML file", "path", path, "err", err)
		return doc
	}
	doc.Text = strings.TrimSpace(string(data))

	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("cannot decode XML, skipping structure", "path", path, "err", err)
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			doc.RootElement = se.Name.Local
			break
		}
	}
	return doc
}

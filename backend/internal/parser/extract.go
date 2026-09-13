package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ErrUnsupportedType is returned when a document is neither a PDF nor a DOCX.
var ErrUnsupportedType = errors.New("parser: unsupported document type")

// TextExtractor turns raw document bytes into plain text.
type TextExtractor interface {
	Extract(data []byte, contentType string) (string, error)
}

// DefaultExtractor extracts text from PDF and DOCX bytes using the document's
// magic bytes (the declared content type is only a fallback, so a mislabeled
// upload cannot smuggle a different format past the API's own checks).
type DefaultExtractor struct{}

// Extract sniffs the document type and returns its plain text.
func (DefaultExtractor) Extract(data []byte, _ string) (string, error) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return extractPDF(data)
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return extractDOCX(data)
	default:
		return "", ErrUnsupportedType
	}
}

// extractDOCX reads word/document.xml from the OOXML package and flattens its
// runs into text, one line per paragraph. A ZIP archive without that part (e.g.
// an XLSX workbook) is rejected as unsupported.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("parser: open docx: %w", err)
	}
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", ErrUnsupportedType
	}
	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("parser: read docx document: %w", err)
	}
	defer rc.Close()

	var out strings.Builder
	dec := xml.NewDecoder(rc)
	inText := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parser: parse docx document: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				out.WriteByte('\t')
			case "br":
				out.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				out.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				out.Write(t)
			}
		}
	}
	return out.String(), nil
}

// extractPDF extracts the plain text of a text-based PDF. Scanned/image-only
// PDFs yield empty text; the caller turns that into a 422 so the user can edit
// their profile manually rather than receiving a silently empty parse.
func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("parser: open pdf: %w", err)
	}
	rd, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("parser: extract pdf text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rd); err != nil {
		return "", fmt.Errorf("parser: read pdf text: %w", err)
	}
	return buf.String(), nil
}

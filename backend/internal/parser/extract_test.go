package parser

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"testing"
)

// docxB64 returns a base64-encoded DOCX for envelope-level tests.
func docxB64(t *testing.T, paragraphs []string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(buildDocx(t, paragraphs))
}

// buildDocx builds a minimal OOXML package with the given paragraphs.
func buildDocx(t *testing.T, paragraphs []string) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
		body.WriteString(html.EscapeString(p))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		body.String() + `</w:body></w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml":   document,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close docx: %v", err)
	}
	return buf.Bytes()
}

// buildPDF builds a single-page PDF with an uncompressed text stream, computing
// a correct cross-reference table so the reader can resolve objects.
func buildPDF(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, 6)
	writeObject := func(n int, body string) {
		offsets[n] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}

	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "+
		"/Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>")
	stream := fmt.Sprintf("BT /F1 24 Tf 72 700 Td (%s) Tj ET", pdfEscape(text))
	writeObject(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	writeObject(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	xref := buf.Len()
	buf.WriteString("xref\n0 6\n")
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return buf.Bytes()
}

func pdfEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(s)
}

func TestExtractDOCX(t *testing.T) {
	docx := buildDocx(t, []string{"Budi Santoso", "Backend Engineer at PT Tokopedia"})
	got, err := DefaultExtractor{}.Extract(docx, "")
	if err != nil {
		t.Fatalf("extract docx: %v", err)
	}
	for _, want := range []string{"Budi Santoso", "Backend Engineer at PT Tokopedia"} {
		if !strings.Contains(got, want) {
			t.Fatalf("docx text %q does not contain %q", got, want)
		}
	}
}

func TestExtractDOCXRejectsNonDocxZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("xl/workbook.xml")
	_, _ = w.Write([]byte("<workbook/>"))
	_ = zw.Close()

	if _, err := (DefaultExtractor{}).Extract(buf.Bytes(), ""); err != ErrUnsupportedType {
		t.Fatalf("got err %v, want ErrUnsupportedType", err)
	}
}

func TestExtractRejectsUnknownType(t *testing.T) {
	if _, err := (DefaultExtractor{}).Extract([]byte("just text"), "text/plain"); err != ErrUnsupportedType {
		t.Fatalf("got err %v, want ErrUnsupportedType", err)
	}
}

func TestExtractPDF(t *testing.T) {
	pdfBytes := buildPDF(t, "Jane Doe Data Analyst Traveloka")
	got, err := DefaultExtractor{}.Extract(pdfBytes, "application/pdf")
	if err != nil {
		t.Fatalf("extract pdf: %v", err)
	}
	if !strings.Contains(got, "Jane Doe") {
		t.Fatalf("pdf text %q does not contain %q", got, "Jane Doe")
	}
}

// TestDOCXRawXMLUnchanged guards the extractor against a regression where XML
// tokens leak into the extracted text.
func TestDOCXRawXMLUnchanged(t *testing.T) {
	docx := buildDocx(t, []string{"Hello"})
	got, _ := DefaultExtractor{}.Extract(docx, "")
	if strings.Contains(got, "<w:") || strings.Contains(got, "xmlns") {
		t.Fatalf("extracted text leaked XML markup: %q", got)
	}
}

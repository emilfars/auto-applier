package parser

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
)

type fakeExtractor struct {
	text string
	err  error
}

func (f fakeExtractor) Extract([]byte, string) (string, error) { return f.text, f.err }

type fakeEngine struct {
	out     Extraction
	err     error
	gotText string
}

func (f *fakeEngine) Extract(_ context.Context, text string) (Extraction, error) {
	f.gotText = text
	return f.out, f.err
}

func TestServiceParse(t *testing.T) {
	eng := &fakeEngine{out: Extraction{Name: "Jane Doe", Skills: []string{"Go"}}}
	svc := NewService(fakeExtractor{text: "resume body"}, eng, 1<<20)
	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("raw docx bytes"))}

	out, err := svc.Parse(context.Background(), doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if out.Name != "Jane Doe" {
		t.Fatalf("unexpected extraction: %+v", out)
	}
	if eng.gotText != "resume body" {
		t.Fatalf("engine got text %q", eng.gotText)
	}
}

func TestServiceRejectsBadBase64(t *testing.T) {
	svc := NewService(fakeExtractor{text: "x"}, &fakeEngine{}, 1<<20)
	_, err := svc.Parse(context.Background(), Document{DocumentB64: "not base64!!"})
	if !errors.Is(err, ErrBadDocument) {
		t.Fatalf("got %v, want ErrBadDocument", err)
	}
}

func TestServiceRejectsOversize(t *testing.T) {
	svc := NewService(fakeExtractor{text: "x"}, &fakeEngine{}, 4)
	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("12345"))}
	if _, err := svc.Parse(context.Background(), doc); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}

func TestServiceRejectsUnsupported(t *testing.T) {
	svc := NewService(fakeExtractor{err: ErrUnsupportedType}, &fakeEngine{}, 1<<20)
	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("hello"))}
	if _, err := svc.Parse(context.Background(), doc); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("got %v, want ErrUnsupportedType", err)
	}
}

func TestServiceRejectsEmptyText(t *testing.T) {
	svc := NewService(fakeExtractor{text: "   \n  "}, &fakeEngine{}, 1<<20)
	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("hello"))}
	if _, err := svc.Parse(context.Background(), doc); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("got %v, want ErrEmptyText", err)
	}
}

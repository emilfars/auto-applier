package parser

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Errors surfaced to the HTTP layer so callers can map them to status codes.
var (
	// ErrTooLarge is returned when the decoded document exceeds the size limit.
	ErrTooLarge = errors.New("parser: document too large")
	// ErrEmptyText is returned when no text could be extracted.
	ErrEmptyText = errors.New("parser: document contains no extractable text")
	// ErrBadDocument is returned when the request document is malformed.
	ErrBadDocument = errors.New("parser: malformed document")
)

// Service orchestrates document decoding, text extraction and model extraction.
type Service struct {
	extractor TextExtractor
	engine    Engine
	maxBytes  int
}

// NewService builds a parser service. maxBytes<=0 falls back to 10 MiB.
func NewService(extractor TextExtractor, engine Engine, maxBytes int) *Service {
	if extractor == nil {
		extractor = DefaultExtractor{}
	}
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	return &Service{extractor: extractor, engine: engine, maxBytes: maxBytes}
}

// Parse decodes a Document and returns the extracted structured résumé.
func (s *Service) Parse(ctx context.Context, doc Document) (Extraction, error) {
	if s.engine == nil {
		return Extraction{}, fmt.Errorf("%w: no engine configured", ErrEngine)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(doc.DocumentB64))
	if err != nil {
		return Extraction{}, fmt.Errorf("%w: document_base64 is not valid base64", ErrBadDocument)
	}
	if len(data) == 0 {
		return Extraction{}, fmt.Errorf("%w: document is empty", ErrBadDocument)
	}
	if len(data) > s.maxBytes {
		return Extraction{}, ErrTooLarge
	}
	text, err := s.extractor.Extract(data, doc.ContentType)
	if err != nil {
		return Extraction{}, err
	}
	if strings.TrimSpace(text) == "" {
		return Extraction{}, ErrEmptyText
	}
	return s.engine.Extract(ctx, text)
}

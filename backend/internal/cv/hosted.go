package cv

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrParserUnavailable is returned when no CV parser is configured. Callers
// surface it as 503 so the feature degrades gracefully rather than crashing.
var ErrParserUnavailable = errors.New("cv: parser not configured")

// disabledParser is used when CV_PARSER_URL is unset. It keeps the upload/edit
// flow fully functional (parsing is an assist, not a hard dependency).
type disabledParser struct{}

// NewDisabledParser returns a Parser that always reports it is unavailable.
func NewDisabledParser() Parser { return disabledParser{} }

func (disabledParser) Parse(context.Context, []byte, string, string) (ParsedCV, error) {
	return ParsedCV{}, ErrParserUnavailable
}

// HostedParser calls a hosted resume-parsing API. The system never depends on a
// specific vendor: the request is a small JSON envelope (base64 document +
// content type) and the response is mapped through hostedResponse below, which
// is the single place to adapt to a given provider's field names.
type HostedParser struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewHostedParser builds a HostedParser. endpoint and apiKey come from
// configuration (CV_PARSER_URL / CV_PARSER_API_KEY). A nil client uses a
// sensible default with a timeout.
func NewHostedParser(endpoint, apiKey string, client *http.Client) *HostedParser {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	cloned := *client
	existingRedirect := cloned.CheckRedirect
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("cv: parser redirect must use https")
		}
		if existingRedirect != nil {
			return existingRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("cv: stopped after 10 parser redirects")
		}
		return nil
	}
	return &HostedParser{endpoint: endpoint, apiKey: apiKey, client: &cloned}
}

type hostedRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	DocumentB64 string `json:"document_base64"`
}

// hostedResponse is the normalized shape we expect back from the parsing API.
// Field names are kept generic; adapting to a concrete vendor means editing the
// json tags (and, if needed, the mapping in toParsedCV) — nothing else.
type hostedResponse struct {
	Data struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`

		Education []struct {
			Institution string `json:"institution"`
			Degree      string `json:"degree"`
			Field       string `json:"field_of_study"`
			StartYear   string `json:"start_year"`
			EndYear     string `json:"end_year"`
		} `json:"education"`

		WorkHistory []struct {
			Company   string `json:"company"`
			Title     string `json:"title"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		} `json:"work_history"`

		Skills []string `json:"skills"`
	} `json:"data"`
}

// toParsedCV maps a raw hosted response into a normalized ParsedCV.
func (h hostedResponse) toParsedCV() ParsedCV {
	p := ParsedCV{
		FullName: h.Data.Name,
		Email:    h.Data.Email,
		Phone:    h.Data.Phone,
		Skills:   h.Data.Skills,
	}
	for _, e := range h.Data.Education {
		p.Education = append(p.Education, EducationEntry{
			Institution: e.Institution,
			Degree:      e.Degree,
			Field:       e.Field,
			StartYear:   e.StartYear,
			EndYear:     e.EndYear,
		})
	}
	for _, w := range h.Data.WorkHistory {
		p.WorkHistory = append(p.WorkHistory, WorkEntry{
			Company:   w.Company,
			Title:     w.Title,
			StartDate: w.StartDate,
			EndDate:   w.EndDate,
		})
	}
	return normalize(p)
}

// Parse sends the document to the hosted API and returns a normalized ParsedCV.
func (h *HostedParser) Parse(ctx context.Context, data []byte, filename, contentType string) (ParsedCV, error) {
	if h.endpoint == "" {
		return ParsedCV{}, ErrParserUnavailable
	}
	endpoint, err := url.Parse(h.endpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return ParsedCV{}, fmt.Errorf("cv: invalid parser endpoint")
	}
	if !secureParserURL(endpoint) {
		return ParsedCV{}, fmt.Errorf("cv: parser endpoint must use https")
	}
	body, err := json.Marshal(hostedRequest{
		Filename:    filename,
		ContentType: contentType,
		DocumentB64: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return ParsedCV{}, fmt.Errorf("cv: marshal parse request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return ParsedCV{}, fmt.Errorf("cv: build parse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return ParsedCV{}, fmt.Errorf("cv: call parse api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ParsedCV{}, fmt.Errorf("cv: parse api status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}

	var parsed hostedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&parsed); err != nil {
		return ParsedCV{}, fmt.Errorf("cv: decode parse response: %w", err)
	}
	return parsed.toParsedCV(), nil
}

func secureParserURL(endpoint *url.URL) bool {
	return endpoint.Scheme == "https" ||
		endpoint.Hostname() == "localhost" ||
		endpoint.Hostname() == "127.0.0.1" ||
		endpoint.Hostname() == "::1"
}

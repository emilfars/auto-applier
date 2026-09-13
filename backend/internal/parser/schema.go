// Package parser implements the CV parser service (M5.5). It accepts the
// envelope documented by backend/internal/cv/hosted.go, extracts text from the
// uploaded document, and structures it with an OpenRouter model.
//
// This is the self-hosted engine referenced in plan.md: the service, not the
// main API, holds OPENROUTER_API_KEY. It is a Go re-implementation of the
// orasik/resume-parser (MIT) approach — text extraction followed by an LLM call
// with a JSON-schema response format — because the repository has no Python
// runtime. The externally visible contract is unchanged.
package parser

// Document is the JSON request envelope posted by cv.HostedParser:
// {filename, content_type, document_base64}.
type Document struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	DocumentB64 string `json:"document_base64"`
}

// Response is the JSON response envelope the API expects: {data:{...}}.
type Response struct {
	Data Extraction `json:"data"`
}

// Extraction is the normalized subset of a résumé the engine returns. The field
// names are the contract fixed by cv.hostedResponse; do not rename them without
// updating the client.
type Extraction struct {
	Name        string           `json:"name"`
	Email       string           `json:"email"`
	Phone       string           `json:"phone"`
	Education   []EducationEntry `json:"education"`
	WorkHistory []WorkEntry      `json:"work_history"`
	Skills      []string         `json:"skills"`
}

// EducationEntry is one schooling record.
type EducationEntry struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Field       string `json:"field_of_study"`
	StartYear   string `json:"start_year"`
	EndYear     string `json:"end_year"`
}

// WorkEntry is one employment record.
type WorkEntry struct {
	Company   string `json:"company"`
	Title     string `json:"title"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// responseFormat returns the OpenRouter response_format asking for JSON matching
// the resume schema. additionalProperties/required are set so strict-capable
// providers return every field; providers without structured-output support
// simply ignore the hint and return JSON in the message content.
func responseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "resume_extraction",
			"schema": resumeSchema(),
		},
	}
}

func resumeSchema() map[string]any {
	str := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  str,
			"email": str,
			"phone": str,
			"education": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"institution":    str,
						"degree":         str,
						"field_of_study": str,
						"start_year":     str,
						"end_year":       str,
					},
					"required":             []string{"institution", "degree", "field_of_study", "start_year", "end_year"},
					"additionalProperties": false,
				},
			},
			"work_history": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"company":    str,
						"title":      str,
						"start_date": str,
						"end_date":   str,
					},
					"required":             []string{"company", "title", "start_date", "end_date"},
					"additionalProperties": false,
				},
			},
			"skills": map[string]any{
				"type":  "array",
				"items": str,
			},
		},
		"required":             []string{"name", "email", "phone", "education", "work_history", "skills"},
		"additionalProperties": false,
	}
}

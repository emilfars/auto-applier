package parser

import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/auto-applier/backend/internal/cv"
)

// TestEnvelopeRoundTripWithHostedParser is the contract test for M5.5: the real
// cv.HostedParser client posts the documented envelope to this service and maps
// the reply back into a normalized ParsedCV. It exercises the real DOCX text
// extractor, the HTTP envelope, and the client mapping in one pass.
func TestEnvelopeRoundTripWithHostedParser(t *testing.T) {
	docx := buildDocx(t, []string{"Jane Doe", "jane.doe@example.com", "Data Analyst at Traveloka"})
	eng := &fakeEngine{out: Extraction{
		Name:  "Jane Doe",
		Email: "jane.doe@example.com",
		Phone: "+62 811 2233 4455",
		Education: []EducationEntry{{
			Institution: "Institut Teknologi Bandung",
			Degree:      "Bachelor of Science",
			Field:       "Computer Science",
			StartYear:   "2012",
			EndYear:     "2016",
		}},
		WorkHistory: []WorkEntry{{
			Company:   "Traveloka",
			Title:     "Data Analyst",
			StartDate: "2017",
			EndDate:   "Present",
		}},
		Skills: []string{"Python", "SQL"},
	}}
	svc := NewService(DefaultExtractor{}, eng, 1<<20)
	srv := httptest.NewServer(NewServer(svc, "envelope-key"))
	defer srv.Close()

	client := cv.NewHostedParser(srv.URL+"/parse", "envelope-key", srv.Client())
	got, err := client.Parse(context.Background(), docx, "resume.docx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	if err != nil {
		t.Fatalf("hosted parse: %v", err)
	}
	want := cv.ParsedCV{
		FullName: "Jane Doe",
		Email:    "jane.doe@example.com",
		Phone:    "+6281122334455",
		Education: []cv.EducationEntry{{
			Institution: "Institut Teknologi Bandung",
			Degree:      "Bachelor of Science",
			Field:       "Computer Science",
			StartYear:   "2012",
			EndYear:     "2016",
		}},
		WorkHistory: []cv.WorkEntry{{
			Company:   "Traveloka",
			Title:     "Data Analyst",
			StartDate: "2017",
			EndDate:   "",
		}},
		Skills: []string{"Python", "SQL"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch\n got:  %+v\n want: %+v", got, want)
	}
	if !strings.Contains(eng.gotText, "jane.doe@example.com") {
		t.Fatalf("engine did not receive extracted text: %q", eng.gotText)
	}
}

// TestEnvelopeAuthRejected confirms the API-side key is enforced end to end.
func TestEnvelopeAuthRejected(t *testing.T) {
	svc := NewService(DefaultExtractor{}, &fakeEngine{}, 1<<20)
	srv := httptest.NewServer(NewServer(svc, "right-key"))
	defer srv.Close()

	client := cv.NewHostedParser(srv.URL+"/parse", "wrong-key", srv.Client())
	if _, err := client.Parse(context.Background(), buildDocx(t, []string{"x"}), "resume.docx", ""); err == nil {
		t.Fatal("expected error for wrong parser key")
	}
}

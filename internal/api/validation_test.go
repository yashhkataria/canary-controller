package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantFields []string
	}{
		{name: "empty object", body: `{}`, wantFields: []string{"namespace", "app", "image", "steps"}},
		{name: "uppercase namespace", body: `{"namespace":"Default","app":"demo","image":"demo:v2","steps":[10]}`, wantFields: []string{"namespace"}},
		{name: "app ends with dash", body: `{"namespace":"default","app":"demo-","image":"demo:v2","steps":[10]}`, wantFields: []string{"app"}},
		{name: "app too long", body: `{"namespace":"default","app":"` + strings.Repeat("a", 64) + `","image":"demo:v2","steps":[10]}`, wantFields: []string{"app"}},
		{name: "image with space", body: `{"namespace":"default","app":"demo","image":"demo v2","steps":[10]}`, wantFields: []string{"image"}},
		{name: "empty steps", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[]}`, wantFields: []string{"steps"}},
		{name: "step not multiple of 10", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[15,100]}`, wantFields: []string{"steps"}},
		{name: "steps not increasing", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[50,50]}`, wantFields: []string{"steps"}},
		{name: "steps decreasing", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[50,10]}`, wantFields: []string{"steps"}},
		{name: "step above 100", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[10,110]}`, wantFields: []string{"steps"}},
		{name: "step zero", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[0,10]}`, wantFields: []string{"steps"}},
		{name: "too many steps", body: `{"namespace":"default","app":"demo","image":"demo:v2","steps":[10,20,30,40,50,60,70,80,90,100,100]}`, wantFields: []string{"steps"}},
		{name: "interval below scrape", body: validBody(`"analysis":{"interval":"1s"}`), wantFields: []string{"analysis.interval"}},
		{name: "checksPerStep zero", body: validBody(`"analysis":{"checksPerStep":0}`), wantFields: []string{"analysis.checksPerStep"}},
		{name: "failureLimit zero", body: validBody(`"analysis":{"failureLimit":0}`), wantFields: []string{"analysis.failureLimit"}},
		{name: "maxErrorRate above 1", body: validBody(`"analysis":{"maxErrorRate":1.5}`), wantFields: []string{"analysis.maxErrorRate"}},
		{name: "errorMargin negative", body: validBody(`"analysis":{"errorMargin":-0.1}`), wantFields: []string{"analysis.errorMargin"}},
		{name: "maxP99Ms zero", body: validBody(`"analysis":{"maxP99Ms":0}`), wantFields: []string{"analysis.maxP99Ms"}},
		{name: "latencyFactor below 1", body: validBody(`"analysis":{"latencyFactor":0.5}`), wantFields: []string{"analysis.latencyFactor"}},
		{name: "minRequests negative", body: validBody(`"analysis":{"minRequests":-1}`), wantFields: []string{"analysis.minRequests"}},
		{name: "maxWait shorter than interval", body: validBody(`"analysis":{"maxWait":"10s"}`), wantFields: []string{"analysis.maxWait"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, h, _ := newTestServer(t)
			rec := postRollout(t, h, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d %s, want 400", rec.Code, rec.Body.String())
			}
			got := decode[errorResponse](t, rec)
			if len(got.Fields) != len(tt.wantFields) {
				t.Errorf("fields = %v, want exactly %v", got.Fields, tt.wantFields)
			}
			for _, f := range tt.wantFields {
				if got.Fields[f] == "" {
					t.Errorf("missing error for %q in %v", f, got.Fields)
				}
			}
			if len(fs.rollouts) != 0 {
				t.Errorf("invalid request stored %d rollouts", len(fs.rollouts))
			}
		})
	}
}

func TestCreateValidSteps(t *testing.T) {
	tests := []string{`[100]`, `[10,50]`, `[10,50,100]`, `[10,20,30,40,50,60,70,80,90,100]`}
	for _, steps := range tests {
		t.Run(steps, func(t *testing.T) {
			_, h, _ := newTestServer(t)
			body := `{"namespace":"default","app":"demo","image":"demo:v2","steps":` + steps + `}`
			if rec := postRollout(t, h, body); rec.Code != http.StatusCreated {
				t.Errorf("status = %d %s, want 201", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateFillsAnalysisDefaults(t *testing.T) {
	_, h, _ := newTestServer(t)
	got := mustCreateRollout(t, h, validBody(`"analysis":{"interval":"30s","maxErrorRate":0.05,"minRequests":0}`))

	want := rollout.DefaultAnalysis()
	want.Interval = rollout.Duration(30 * time.Second)
	want.MaxErrorRate = 0.05
	want.MinRequests = 0
	if got.Analysis != want {
		t.Errorf("analysis = %+v\nwant       %+v", got.Analysis, want)
	}
}

func TestCreateRequestErrors(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantMsg     string
	}{
		{name: "wrong content type", body: validBody(""), contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType, wantMsg: "Content-Type"},
		{name: "json with charset", body: validBody(""), contentType: "application/json; charset=utf-8", wantStatus: http.StatusCreated},
		{name: "malformed json", body: `{"namespace":`, wantStatus: http.StatusBadRequest, wantMsg: "invalid JSON"},
		{name: "unknown field", body: validBody(`"maxErrorRte":0.5`), wantStatus: http.StatusBadRequest, wantMsg: "unknown field"},
		{name: "unknown analysis field", body: validBody(`"analysis":{"maxErrorRte":0.5}`), wantStatus: http.StatusBadRequest, wantMsg: "unknown field"},
		{name: "two objects", body: validBody("") + `{}`, wantStatus: http.StatusBadRequest, wantMsg: "single JSON object"},
		{name: "bad duration", body: validBody(`"analysis":{"interval":"soon"}`), wantStatus: http.StatusBadRequest, wantMsg: "invalid duration"},
		{name: "too large", body: validBody(`"image":"` + strings.Repeat("x", maxBodyBytes) + `"`), wantStatus: http.StatusRequestEntityTooLarge, wantMsg: "at most"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, h, _ := newTestServer(t)
			headers := map[string]string{}
			if tt.contentType != "" {
				headers["Content-Type"] = tt.contentType
			}
			rec := send(t, h, http.MethodPost, "/v1/create-rollout", tt.body, headers)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			if tt.wantMsg != "" {
				if msg := decode[errorResponse](t, rec).Error; !strings.Contains(msg, tt.wantMsg) {
					t.Errorf("error = %q, want it to mention %q", msg, tt.wantMsg)
				}
			}
		})
	}
}

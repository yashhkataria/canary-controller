package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		sent     string
		wantSame bool
	}{
		{name: "reuses a valid id", sent: "abc-123.x_y", wantSame: true},
		{name: "replaces an id with spaces", sent: "abc 123"},
		{name: "replaces an id that is too long", sent: strings.Repeat("a", 65)},
		{name: "generates one when missing", sent: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, h, _ := newTestServer(t)
			headers := map[string]string{}
			if tt.sent != "" {
				headers[headerRequestID] = tt.sent
			}
			got := send(t, h, http.MethodGet, "/healthz", "", headers).Header().Get(headerRequestID)
			if tt.wantSame && got != tt.sent {
				t.Errorf("request id = %q, want %q", got, tt.sent)
			}
			if !tt.wantSame && (got == "" || got == tt.sent) {
				t.Errorf("request id = %q, want a newly generated one", got)
			}
		})
	}
}

func TestRequestLogLine(t *testing.T) {
	_, h, logs := newTestServer(t)
	send(t, h, http.MethodGet, "/v1/rollouts/"+bson.NewObjectID().Hex(), "", map[string]string{headerRequestID: "req-1"})

	line := findLog(t, logs.String(), "request")
	want := map[string]any{"method": "GET", "status": float64(404), "request_id": "req-1", "level": "INFO"}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("log %s = %v, want %v (line %v)", k, line[k], v, line)
		}
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Errorf("log line has no duration_ms: %v", line)
	}
}

func TestProbesLogAtDebug(t *testing.T) {
	_, h, logs := newTestServer(t)
	send(t, h, http.MethodGet, "/healthz", "", nil)
	if level := findLog(t, logs.String(), "request")["level"]; level != "DEBUG" {
		t.Errorf("probe request logged at %v, want DEBUG", level)
	}
}

func TestPanicBecomes500(t *testing.T) {
	fs, h, logs := newTestServer(t)
	fs.panicOnGet = true

	rec := send(t, h, http.MethodGet, "/v1/rollouts/"+bson.NewObjectID().Hex(), "", map[string]string{headerRequestID: "req-panic"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	panicLine := findLog(t, logs.String(), "panic in handler")
	if panicLine["request_id"] != "req-panic" {
		t.Errorf("panic log has request_id %v, want req-panic", panicLine["request_id"])
	}
	if status := findLog(t, logs.String(), "request")["status"]; status != float64(500) {
		t.Errorf("request log status = %v, want 500", status)
	}
}

func findLog(t *testing.T, logs, msg string) map[string]any {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(logs))
	for sc.Scan() {
		var line map[string]any
		if json.Unmarshal(sc.Bytes(), &line) == nil && line["msg"] == msg {
			return line
		}
	}
	t.Fatalf("no log line with msg %q in:\n%s", msg, logs)
	return nil
}

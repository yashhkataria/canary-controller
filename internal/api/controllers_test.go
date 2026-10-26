package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

func TestCreateRollout(t *testing.T) {
	fs, h, _ := newTestServer(t)
	rec := postRollout(t, h, validBody(""))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d %s, want 201", rec.Code, rec.Body.String())
	}
	got := decode[rollout.Rollout](t, rec)
	if got.Phase != rollout.PhasePending || got.CanaryImage != "demo:v2" || got.StableImage != "" || got.CurrentStep != 0 {
		t.Errorf("created = %+v; want Pending, canaryImage demo:v2, no stableImage, step 0", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/v1/rollouts/"+got.ID.Hex() {
		t.Errorf("Location = %q", loc)
	}
	if len(fs.events) != 1 || fs.events[0].Type != rollout.EventCreated || fs.events[0].RolloutID != got.ID {
		t.Errorf("events = %+v, want one Created event for the rollout", fs.events)
	}
}

func TestCreateConflict(t *testing.T) {
	tests := []struct {
		name       string
		second     string
		wantStatus int
	}{
		{name: "same app", second: validBody(""), wantStatus: http.StatusConflict},
		{name: "other app", second: `{"namespace":"default","app":"other","image":"demo:v2","steps":[10]}`, wantStatus: http.StatusCreated},
		{name: "other namespace", second: `{"namespace":"staging","app":"demo","image":"demo:v2","steps":[10]}`, wantStatus: http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, h, _ := newTestServer(t)
			mustCreateRollout(t, h, validBody(""))
			if rec := postRollout(t, h, tt.second); rec.Code != tt.wantStatus {
				t.Errorf("second create = %d %s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
		})
	}
}

func TestGetRollout(t *testing.T) {
	_, h, _ := newTestServer(t)
	created := mustCreateRollout(t, h, validBody(""))

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "found", path: "/v1/rollouts/" + created.ID.Hex(), wantStatus: http.StatusOK},
		{name: "missing", path: "/v1/rollouts/" + bson.NewObjectID().Hex(), wantStatus: http.StatusNotFound},
		{name: "invalid id", path: "/v1/rollouts/not-an-id", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, h, http.MethodGet, tt.path, "", nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && decode[rollout.Rollout](t, rec).ID != created.ID {
				t.Errorf("got a different rollout")
			}
		})
	}
}

func TestListRollouts(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantFilter store.ListFilter
		wantField  string
	}{
		{name: "app only", query: "?app=demo", wantStatus: http.StatusOK, wantFilter: store.ListFilter{App: "demo", Limit: store.DefaultListLimit}},
		{name: "all params", query: "?app=demo&namespace=default&limit=5", wantStatus: http.StatusOK, wantFilter: store.ListFilter{App: "demo", Namespace: "default", Limit: 5}},
		{name: "missing app", query: "", wantStatus: http.StatusBadRequest, wantField: "app"},
		{name: "bad namespace", query: "?app=demo&namespace=Bad", wantStatus: http.StatusBadRequest, wantField: "namespace"},
		{name: "limit zero", query: "?app=demo&limit=0", wantStatus: http.StatusBadRequest, wantField: "limit"},
		{name: "limit too big", query: "?app=demo&limit=101", wantStatus: http.StatusBadRequest, wantField: "limit"},
		{name: "limit not a number", query: "?app=demo&limit=ten", wantStatus: http.StatusBadRequest, wantField: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, h, _ := newTestServer(t)
			mustCreateRollout(t, h, validBody(""))
			rec := send(t, h, http.MethodGet, "/v1/rollouts"+tt.query, "", nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			if tt.wantField != "" {
				if decode[errorResponse](t, rec).Fields[tt.wantField] == "" {
					t.Errorf("want an error for %q, got %s", tt.wantField, rec.Body.String())
				}
				return
			}
			if fs.lastFilter != tt.wantFilter {
				t.Errorf("filter = %+v, want %+v", fs.lastFilter, tt.wantFilter)
			}
			if items := decode[listResponse[rollout.Rollout]](t, rec).Items; len(items) != 1 {
				t.Errorf("items = %d, want 1", len(items))
			}
		})
	}
}

func TestListEvents(t *testing.T) {
	_, h, _ := newTestServer(t)
	created := mustCreateRollout(t, h, validBody(""))

	rec := send(t, h, http.MethodGet, "/v1/rollouts/"+created.ID.Hex()+"/events", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s, want 200", rec.Code, rec.Body.String())
	}
	items := decode[listResponse[rollout.Event]](t, rec).Items
	if len(items) != 1 || items[0].Type != rollout.EventCreated {
		t.Errorf("events = %+v, want one Created event", items)
	}

	if rec := send(t, h, http.MethodGet, "/v1/rollouts/"+bson.NewObjectID().Hex()+"/events", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("events of missing rollout = %d, want 404", rec.Code)
	}
}

func TestEmptyListsAreArrays(t *testing.T) {
	_, h, _ := newTestServer(t)
	rec := send(t, h, http.MethodGet, "/v1/rollouts?app=nothing", "", nil)
	if body := rec.Body.String(); body != "{\"items\":[]}\n" {
		t.Errorf("body = %q, want an empty items array", body)
	}
}

func TestAbortAndPromoteByPhase(t *testing.T) {
	phases := []rollout.Phase{
		rollout.PhasePending, rollout.PhaseProgressing, rollout.PhaseAnalyzing, rollout.PhasePromoting,
		rollout.PhaseRollingBack, rollout.PhaseSucceeded, rollout.PhaseRolledBack,
	}
	want := map[string]map[rollout.Phase]int{
		"abort": {
			rollout.PhasePending: 202, rollout.PhaseProgressing: 202, rollout.PhaseAnalyzing: 202, rollout.PhasePromoting: 202,
			rollout.PhaseRollingBack: 409, rollout.PhaseSucceeded: 409, rollout.PhaseRolledBack: 409,
		},
		"promote": {
			rollout.PhasePending: 409, rollout.PhaseProgressing: 202, rollout.PhaseAnalyzing: 202, rollout.PhasePromoting: 409,
			rollout.PhaseRollingBack: 409, rollout.PhaseSucceeded: 409, rollout.PhaseRolledBack: 409,
		},
	}
	nextPhase := map[string]rollout.Phase{"abort": rollout.PhaseRollingBack, "promote": rollout.PhasePromoting}
	nextEvent := map[string]string{"abort": rollout.EventRollingBack, "promote": rollout.EventPromoting}

	for _, action := range []string{"abort", "promote"} {
		for _, phase := range phases {
			t.Run(action+" from "+string(phase), func(t *testing.T) {
				fs, h, _ := newTestServer(t)
				created := mustCreateRollout(t, h, validBody(""))
				fs.setPhase(created.ID, phase)

				rec := send(t, h, http.MethodPost, "/v1/rollouts/"+created.ID.Hex()+"/"+action, "", nil)
				if rec.Code != want[action][phase] {
					t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), want[action][phase])
				}
				stored, _ := fs.GetRollout(context.Background(), created.ID)
				if rec.Code != http.StatusAccepted {
					if stored.Phase != phase {
						t.Errorf("rejected request still changed the phase to %s", stored.Phase)
					}
					return
				}
				got := decode[rollout.Rollout](t, rec)
				if got.Phase != nextPhase[action] || stored.Phase != nextPhase[action] || got.WaitingSince == nil {
					t.Errorf("phase = %s (stored %s), want %s with waitingSince set", got.Phase, stored.Phase, nextPhase[action])
				}
				if action == "abort" && got.Reason != abortReason {
					t.Errorf("reason = %q", got.Reason)
				}
				if last := fs.events[len(fs.events)-1]; last.Type != nextEvent[action] {
					t.Errorf("last event = %s, want %s", last.Type, nextEvent[action])
				}
			})
		}
	}
}

func TestAbortRetriesOnConflict(t *testing.T) {
	tests := []struct {
		name       string
		conflicts  int
		wantStatus int
	}{
		{name: "succeeds after two conflicts", conflicts: 2, wantStatus: http.StatusAccepted},
		{name: "gives up after three", conflicts: 3, wantStatus: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, h, _ := newTestServer(t)
			created := mustCreateRollout(t, h, validBody(""))
			fs.conflictsLeft = tt.conflicts
			if rec := send(t, h, http.MethodPost, "/v1/rollouts/"+created.ID.Hex()+"/abort", "", nil); rec.Code != tt.wantStatus {
				t.Errorf("status = %d %s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
		})
	}
}

func TestAbortUnknownRollout(t *testing.T) {
	_, h, _ := newTestServer(t)
	if rec := send(t, h, http.MethodPost, "/v1/rollouts/"+bson.NewObjectID().Hex()+"/abort", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("abort of missing rollout = %d, want 404", rec.Code)
	}
	if rec := send(t, h, http.MethodPost, "/v1/rollouts/nope/promote", "", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("promote with bad id = %d, want 400", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodDelete, path: "/v1/rollouts"},
		{method: http.MethodPost, path: "/v1/rollouts", body: validBody("")},
		{method: http.MethodGet, path: "/v1/create-rollout"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			fs, h, _ := newTestServer(t)
			if rec := send(t, h, tt.method, tt.path, tt.body, nil); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want 405", rec.Code)
			}
			if len(fs.rollouts) != 0 {
				t.Errorf("stored %d rollouts, want 0", len(fs.rollouts))
			}
		})
	}
}

func TestHealthAndReady(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		pingErr    error
		wantStatus int
	}{
		{name: "healthz", path: "/healthz", wantStatus: http.StatusOK},
		{name: "healthz ignores mongodb", path: "/healthz", pingErr: errors.New("down"), wantStatus: http.StatusOK},
		{name: "ready", path: "/readyz", wantStatus: http.StatusOK},
		{name: "not ready", path: "/readyz", pingErr: errors.New("down"), wantStatus: http.StatusServiceUnavailable},
		{name: "metrics", path: "/metrics", wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, h, _ := newTestServer(t)
			fs.pingErr = tt.pingErr
			if rec := send(t, h, http.MethodGet, tt.path, "", nil); rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

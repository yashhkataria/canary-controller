package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

type fakeStore struct {
	mu            sync.Mutex
	rollouts      map[bson.ObjectID]rollout.Rollout
	events        []rollout.Event
	lastFilter    store.ListFilter
	pingErr       error
	panicOnGet    bool
	conflictsLeft int
}

func newFakeStore() *fakeStore {
	return &fakeStore{rollouts: map[bson.ObjectID]rollout.Rollout{}}
}

func (f *fakeStore) CreateRollout(_ context.Context, r rollout.Rollout) (rollout.Rollout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.rollouts {
		if existing.Active && existing.Namespace == r.Namespace && existing.App == r.App {
			return rollout.Rollout{}, store.ErrActiveRolloutExists
		}
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	r.ID = bson.NewObjectID()
	r.Version = 1
	r.Active = true
	r.CreatedAt = now
	r.UpdatedAt = now
	f.rollouts[r.ID] = r
	return r, nil
}

func (f *fakeStore) GetRollout(_ context.Context, id bson.ObjectID) (rollout.Rollout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicOnGet {
		panic("boom")
	}
	r, ok := f.rollouts[id]
	if !ok {
		return rollout.Rollout{}, store.ErrNotFound
	}
	return r, nil
}

func (f *fakeStore) ListRollouts(_ context.Context, filter store.ListFilter) ([]rollout.Rollout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	out := []rollout.Rollout{}
	for _, r := range f.rollouts {
		if r.App == filter.App && (filter.Namespace == "" || r.Namespace == filter.Namespace) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateRollout(_ context.Context, r rollout.Rollout) (rollout.Rollout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conflictsLeft > 0 {
		f.conflictsLeft--
		return rollout.Rollout{}, store.ErrVersionConflict
	}
	cur, ok := f.rollouts[r.ID]
	if !ok {
		return rollout.Rollout{}, store.ErrNotFound
	}
	if cur.Version != r.Version {
		return rollout.Rollout{}, store.ErrVersionConflict
	}
	r.Version++
	f.rollouts[r.ID] = r
	return r, nil
}

func (f *fakeStore) setPhase(id bson.ObjectID, p rollout.Phase) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.rollouts[id]
	r.Phase = p
	if p.Terminal() {
		r.Active = false
	}
	f.rollouts[id] = r
}

func (f *fakeStore) AppendEvent(_ context.Context, e rollout.Event) (rollout.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e.ID = bson.NewObjectID()
	f.events = append(f.events, e)
	return e, nil
}

func (f *fakeStore) ListEvents(_ context.Context, id bson.ObjectID) ([]rollout.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []rollout.Event{}
	for _, e := range f.events {
		if e.RolloutID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeStore) Ping(context.Context) error {
	return f.pingErr
}

func newTestServer(t *testing.T) (*fakeStore, http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	fs := newFakeStore()
	return fs, New(fs, logger).Handler(), &logs
}

func send(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postRollout(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, h, http.MethodPost, "/v1/create-rollout", body, nil)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func validBody(overrides string) string {
	base := `"namespace":"default","app":"demo","image":"demo:v2","steps":[10,50,100]`
	if overrides == "" {
		return "{" + base + "}"
	}
	return "{" + base + "," + overrides + "}"
}

func mustCreateRollout(t *testing.T, h http.Handler, body string) rollout.Rollout {
	t.Helper()
	rec := postRollout(t, h, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s, want 201", rec.Code, rec.Body.String())
	}
	return decode[rollout.Rollout](t, rec)
}

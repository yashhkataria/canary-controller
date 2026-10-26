package store_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	in := newRollout("default", "demo")
	in.Analysis.Interval = rollout.Duration(30 * time.Second)
	created := mustCreate(t, s, in)

	if created.ID.IsZero() || created.Version != 1 || !created.Active || created.CreatedAt.IsZero() {
		t.Fatalf("created = %+v; want id set, version 1, active, createdAt set", created)
	}

	got, err := s.GetRollout(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if !reflect.DeepEqual(got, created) {
		t.Errorf("round trip mismatch\n got  %+v\n want %+v", got, created)
	}
	if got.Analysis.Interval.Std() != 30*time.Second {
		t.Errorf("interval = %v, want 30s", got.Analysis.Interval)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	if _, err := s.GetRollout(context.Background(), bson.NewObjectID()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestOneActiveRolloutPerApp(t *testing.T) {
	tests := []struct {
		name    string
		second  rollout.Rollout
		wantErr error
	}{
		{name: "same app and namespace", second: newRollout("default", "demo"), wantErr: store.ErrActiveRolloutExists},
		{name: "same app, other namespace", second: newRollout("staging", "demo")},
		{name: "other app, same namespace", second: newRollout("default", "other")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(t)
			mustCreate(t, s, newRollout("default", "demo"))

			_, err := s.CreateRollout(context.Background(), tt.second)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("second create err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewRolloutAllowedAfterPreviousFinishes(t *testing.T) {
	s := newStore(t)
	first := mustCreate(t, s, newRollout("default", "demo"))
	finish(t, s, first)

	if _, err := s.CreateRollout(context.Background(), newRollout("default", "demo")); err != nil {
		t.Errorf("create after the first finished: %v", err)
	}
}

func TestStaleVersionUpdateRejected(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	created := mustCreate(t, s, newRollout("default", "demo"))

	ownerA := created
	ownerB := created

	ownerA.Phase = rollout.PhaseProgressing
	ownerA.Owner = "replica-a"
	updated, err := s.UpdateRollout(ctx, ownerA)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("version after first update = %d, want 2", updated.Version)
	}

	ownerB.Phase = rollout.PhaseRollingBack
	ownerB.Owner = "replica-b"
	if _, err := s.UpdateRollout(ctx, ownerB); !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("stale update err = %v, want ErrVersionConflict", err)
	}

	got, err := s.GetRollout(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.Phase != rollout.PhaseProgressing || got.Owner != "replica-a" || got.Version != 2 {
		t.Errorf("after stale write: phase=%s owner=%s version=%d; want Progressing, replica-a, 2",
			got.Phase, got.Owner, got.Version)
	}
}

func TestUpdateIncrementsVersion(t *testing.T) {
	s := newStore(t)
	r := mustCreate(t, s, newRollout("default", "demo"))

	for want := int64(2); want <= 5; want++ {
		r.CurrentStep++
		next, err := s.UpdateRollout(context.Background(), r)
		if err != nil {
			t.Fatalf("update to version %d: %v", want, err)
		}
		if next.Version != want || next.CurrentStep != r.CurrentStep {
			t.Fatalf("version=%d step=%d; want %d, %d", next.Version, next.CurrentStep, want, r.CurrentStep)
		}
		if !next.UpdatedAt.After(r.UpdatedAt) && !next.UpdatedAt.Equal(r.UpdatedAt) {
			t.Errorf("updatedAt went backwards: %v -> %v", r.UpdatedAt, next.UpdatedAt)
		}
		r = next
	}
}

func TestUpdateMissingRollout(t *testing.T) {
	s := newStore(t)
	r := newRollout("default", "demo")
	r.ID = bson.NewObjectID()
	r.Version = 1
	if _, err := s.UpdateRollout(context.Background(), r); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReactivatingWhileAnotherIsActiveIsRejected(t *testing.T) {
	s := newStore(t)
	old := finish(t, s, mustCreate(t, s, newRollout("default", "demo")))
	mustCreate(t, s, newRollout("default", "demo"))

	old.Active = true
	if _, err := s.UpdateRollout(context.Background(), old); !errors.Is(err, store.ErrActiveRolloutExists) {
		t.Errorf("err = %v, want ErrActiveRolloutExists", err)
	}
}

func TestListRollouts(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	var ids []bson.ObjectID
	for range 3 {
		r := mustCreate(t, s, newRollout("default", "demo"))
		finish(t, s, r)
		ids = append(ids, r.ID)
		time.Sleep(2 * time.Millisecond)
	}
	mustCreate(t, s, newRollout("default", "other"))
	mustCreate(t, s, newRollout("staging", "demo"))

	tests := []struct {
		name   string
		filter store.ListFilter
		want   []bson.ObjectID
	}{
		{name: "newest first", filter: store.ListFilter{Namespace: "default", App: "demo"}, want: []bson.ObjectID{ids[2], ids[1], ids[0]}},
		{name: "limit", filter: store.ListFilter{Namespace: "default", App: "demo", Limit: 2}, want: []bson.ObjectID{ids[2], ids[1]}},
		{name: "unknown app", filter: store.ListFilter{App: "nope"}, want: []bson.ObjectID{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ListRollouts(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListRollouts: %v", err)
			}
			gotIDs := []bson.ObjectID{}
			for _, r := range got {
				gotIDs = append(gotIDs, r.ID)
			}
			if !slices.Equal(gotIDs, tt.want) {
				t.Errorf("ids = %v, want %v", gotIDs, tt.want)
			}
		})
	}

	all, err := s.ListRollouts(ctx, store.ListFilter{App: "demo"})
	if err != nil {
		t.Fatalf("ListRollouts without namespace: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("app=demo across namespaces = %d rollouts, want 4", len(all))
	}
}

func TestEventsInOrder(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	r := mustCreate(t, s, newRollout("default", "demo"))
	other := mustCreate(t, s, newRollout("default", "other"))

	base := time.Now().UTC().Truncate(time.Millisecond)
	for i, typ := range []string{"Second", "First", "Third"} {
		offset := map[string]int{"First": 0, "Second": 1, "Third": 2}[typ]
		if _, err := s.AppendEvent(ctx, rollout.Event{
			RolloutID: r.ID,
			At:        base.Add(time.Duration(offset) * time.Second),
			Type:      typ,
			Message:   "event",
			Metrics:   map[string]float64{"i": float64(i)},
		}); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
	if _, err := s.AppendEvent(ctx, rollout.Event{RolloutID: other.ID, Type: "Elsewhere"}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	got, err := s.ListEvents(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	var types []string
	for _, e := range got {
		types = append(types, e.Type)
	}
	if want := []string{"First", "Second", "Third"}; !slices.Equal(types, want) {
		t.Errorf("event order = %v, want %v", types, want)
	}
	if got[0].Metrics["i"] != 1 {
		t.Errorf("metrics not stored: %v", got[0].Metrics)
	}
}

func TestIndexesExist(t *testing.T) {
	_, db := newStoreWithDB(t)
	got := indexNames(t, db)
	want := map[string][]string{
		"rollouts": {store.IndexOneActivePerApp, store.IndexActiveUpdated, store.IndexAppHistory},
		"events":   {store.IndexEventsByRollout},
	}
	for coll, names := range want {
		for _, name := range names {
			if !slices.Contains(got[coll], name) {
				t.Errorf("collection %s is missing index %s (has %v)", coll, name, got[coll])
			}
		}
	}
}

func TestListActiveAndStepTimes(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	done := finish(t, s, mustCreate(t, s, newRollout("default", "old")))
	a := mustCreate(t, s, newRollout("default", "a"))
	time.Sleep(2 * time.Millisecond)
	b := mustCreate(t, s, newRollout("default", "b"))
	time.Sleep(2 * time.Millisecond)

	started := time.Now().UTC().Truncate(time.Millisecond)
	a.Phase = rollout.PhaseAnalyzing
	a.StepStartedAt = &started
	a.WaitingSince = &started
	a, err := s.UpdateRollout(ctx, a)
	if err != nil {
		t.Fatalf("UpdateRollout: %v", err)
	}

	active, err := s.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	var ids []bson.ObjectID
	for _, r := range active {
		ids = append(ids, r.ID)
	}
	if want := []bson.ObjectID{b.ID, a.ID}; !slices.Equal(ids, want) {
		t.Errorf("active ids = %v, want %v (least recently updated first, %v excluded)", ids, want, done.ID)
	}

	got, err := s.GetRollout(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetRollout: %v", err)
	}
	if got.StepStartedAt == nil || !got.StepStartedAt.Equal(started) || got.WaitingSince == nil || !got.WaitingSince.Equal(started) {
		t.Errorf("step times not stored: stepStartedAt=%v waitingSince=%v, want %v", got.StepStartedAt, got.WaitingSince, started)
	}

	a.StepStartedAt = nil
	a.WaitingSince = nil
	if a, err = s.UpdateRollout(ctx, a); err != nil {
		t.Fatalf("clear step times: %v", err)
	}
	if a.StepStartedAt != nil || a.WaitingSince != nil {
		t.Errorf("step times not cleared: %v %v", a.StepStartedAt, a.WaitingSince)
	}
}

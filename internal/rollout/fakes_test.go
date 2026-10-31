package rollout

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/yashhkataria/canary-controller/internal/k8s"
)

var deploymentsGVR = appsv1.SchemeGroupVersion.WithResource("deployments")

type memStore struct {
	mu       sync.Mutex
	rollouts map[bson.ObjectID]Rollout
	events   []Event
}

func (m *memStore) ListActive(context.Context) ([]Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Rollout{}
	for _, r := range m.rollouts {
		if r.Active {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) UpdateRollout(_ context.Context, r Rollout) (Rollout, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rollouts[r.ID]
	if !ok {
		return Rollout{}, ErrNotFound
	}
	if cur.Version != r.Version {
		return Rollout{}, ErrVersionConflict
	}
	r.Version++
	m.rollouts[r.ID] = r
	return r, nil
}

func (m *memStore) AppendEvent(_ context.Context, e Event) (Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return e, nil
}

func (m *memStore) eventsOf(typ string) []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Event{}
	for _, e := range m.events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func (m *memStore) phaseEventTypes() []string {
	out := []string{}
	for _, t := range m.eventTypes() {
		if t != EventCheck {
			out = append(out, t)
		}
	}
	return out
}

func (m *memStore) eventTypes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for _, e := range m.events {
		out = append(out, e.Type)
	}
	return out
}

type scriptedAnalyzer struct {
	mu       sync.Mutex
	verdicts []Verdict
	fallback Verdict
	err      error
	calls    int
}

func (a *scriptedAnalyzer) Check(_ context.Context, _ Rollout, now time.Time) (LastCheck, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if a.err != nil {
		return LastCheck{}, a.err
	}
	v := a.fallback
	if len(a.verdicts) > 0 {
		v, a.verdicts = a.verdicts[0], a.verdicts[1:]
	}
	return LastCheck{
		At:      now,
		Verdict: v,
		Reason:  "scripted " + string(v),
		Stable:  TrackMetrics{ErrorRate: 0.001, P99Ms: 5, Requests: 900},
		Canary:  TrackMetrics{ErrorRate: 0.002, P99Ms: 6, Requests: 100},
	}, nil
}

func (a *scriptedAnalyzer) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

type harness struct {
	t        *testing.T
	cs       *fake.Clientset
	store    *memStore
	engine   *Engine
	analyzer *scriptedAnalyzer
	now      time.Time
	id       bson.ObjectID
}

func newHarness(t *testing.T, steps []int, objects ...runtime.Object) *harness {
	t.Helper()
	if objects == nil {
		objects = []runtime.Object{testDeployment("demo-stable", 10, "demo:v1"), testDeployment("demo-canary", 0, "demo:v1")}
	}
	h := &harness{
		t:     t,
		cs:    fake.NewClientset(objects...),
		store: &memStore{rollouts: map[bson.ObjectID]Rollout{}},
		now:   time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
		id:    bson.NewObjectID(),
	}
	a := DefaultAnalysis()
	a.Interval = Duration(10 * time.Second)
	a.ChecksPerStep = 2
	a.MaxWait = Duration(5 * time.Minute)
	h.store.rollouts[h.id] = Rollout{
		ID: h.id, Namespace: "default", App: "demo", CanaryImage: "demo:v2",
		Steps: steps, Phase: PhasePending, Analysis: a, Active: true, Version: 1,
	}
	h.analyzer = &scriptedAnalyzer{fallback: VerdictPass}
	h.engine = NewEngine(h.store, k8s.NewFromClientset(h.cs), h.analyzer, EngineOptions{
		Owner:  "controller-0",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return h.now },
	})
	h.guardCapacity()
	return h
}

func testDeployment(name string, replicas int32, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "demo", Image: image}}}},
		},
		Status: appsv1.DeploymentStatus{Replicas: replicas, ReadyReplicas: replicas, UpdatedReplicas: replicas, AvailableReplicas: replicas},
	}
}

func (h *harness) guardCapacity() {
	h.cs.PrependReactor("patch", "deployments", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.PatchAction)
		if p.GetSubresource() != "scale" {
			return false, nil, nil
		}
		var n int32
		if _, err := fmtSscan(string(p.GetPatch()), &n); err != nil {
			h.t.Errorf("unexpected scale patch %s", p.GetPatch())
			return false, nil, nil
		}
		self := h.rawDeployment(p.GetName())
		if n >= *self.Spec.Replicas {
			return false, nil, nil
		}
		other := "demo-canary"
		if p.GetName() == "demo-canary" {
			other = "demo-stable"
		}
		if ready := h.rawDeployment(other).Status.ReadyReplicas; n+ready < TotalReplicas {
			h.t.Errorf("capacity would drop: scaling %s down to %d while %s has only %d ready pods", p.GetName(), n, other, ready)
		}
		return false, nil, nil
	})
}

func (h *harness) rawDeployment(name string) *appsv1.Deployment {
	obj, err := h.cs.Tracker().Get(appsv1.SchemeGroupVersion.WithResource("deployments"), "default", name)
	if err != nil {
		h.t.Fatalf("get %s from tracker: %v", name, err)
	}
	return obj.(*appsv1.Deployment)
}

func (h *harness) deployment(name string) (replicas int32, image string) {
	d := h.rawDeployment(name)
	return *d.Spec.Replicas, d.Spec.Template.Spec.Containers[0].Image
}

func (h *harness) markReady() {
	h.t.Helper()
	for _, name := range []string{"demo-stable", "demo-canary"} {
		d := h.rawDeployment(name).DeepCopy()
		n := *d.Spec.Replicas
		d.Status = appsv1.DeploymentStatus{ObservedGeneration: d.Generation, Replicas: n, ReadyReplicas: n, UpdatedReplicas: n, AvailableReplicas: n}
		if _, err := h.cs.AppsV1().Deployments("default").UpdateStatus(context.Background(), d, metav1.UpdateOptions{}); err != nil {
			h.t.Fatalf("update status of %s: %v", name, err)
		}
	}
}

func (h *harness) tick() Rollout {
	h.t.Helper()
	h.engine.Tick(context.Background())
	return h.rollout()
}

func (h *harness) rollout() Rollout {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	return h.store.rollouts[h.id]
}

func (h *harness) setPhase(p Phase) {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	r := h.store.rollouts[h.id]
	r.Phase = p
	now := h.now
	r.WaitingSince = &now
	r.Version++
	h.store.rollouts[h.id] = r
}

func (h *harness) setAnalysis(mutate func(*Analysis)) {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	r := h.store.rollouts[h.id]
	mutate(&r.Analysis)
	h.store.rollouts[h.id] = r
}

func (h *harness) tickAt(offset time.Duration) Rollout {
	h.t.Helper()
	h.now = h.rollout().StepStartedAt.Add(offset)
	return h.tick()
}

func (h *harness) patchCount() int {
	n := 0
	for _, a := range h.cs.Actions() {
		if a.GetVerb() == "patch" {
			n++
		}
	}
	return n
}

func (h *harness) runUntilStep(step int, phase Phase, maxTicks int) Rollout {
	h.t.Helper()
	for i := 0; i < maxTicks; i++ {
		r := h.tick()
		if r.CurrentStep == step && r.Phase == phase {
			return r
		}
		h.markReady()
		h.now = h.now.Add(5 * time.Second)
	}
	h.t.Fatalf("did not reach step %d %s within %d ticks; stuck in %s (step %d)", step, phase, maxTicks, h.rollout().Phase, h.rollout().CurrentStep)
	return Rollout{}
}

func (h *harness) runUntil(phase Phase, maxTicks int) Rollout {
	h.t.Helper()
	for i := 0; i < maxTicks; i++ {
		r := h.tick()
		if r.Phase == phase {
			return r
		}
		h.markReady()
		h.now = h.now.Add(5 * time.Second)
	}
	h.t.Fatalf("did not reach %s within %d ticks; stuck in %s (step %d)", phase, maxTicks, h.rollout().Phase, h.rollout().CurrentStep)
	return Rollout{}
}

func fmtSscan(patch string, n *int32) (int, error) {
	return fmt.Sscanf(strings.TrimSpace(patch), `{"spec":{"replicas":%d}}`, n)
}

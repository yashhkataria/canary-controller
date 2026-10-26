package k8s

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestStatus(t *testing.T) {
	cs := fake.NewClientset(deployment("demo-stable", 10))
	got, err := NewFromClientset(cs).Status(context.Background(), "default", "demo-stable", "demo")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := DeploymentStatus{
		Name: "demo-stable", Container: "demo", Image: "demo:v1",
		Desired: 10, Replicas: 10, Ready: 10, Updated: 10, Available: 10,
		Generation: 3, ObservedGeneration: 3,
	}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestSetReplicasPatchesScaleSubresource(t *testing.T) {
	cs := fake.NewClientset(deployment("demo-canary", 0))
	c := NewFromClientset(cs)

	if err := c.SetReplicas(context.Background(), "default", "demo-canary", 3); err != nil {
		t.Fatalf("SetReplicas: %v", err)
	}

	var patches []k8stesting.PatchAction
	for _, a := range cs.Actions() {
		if p, ok := a.(k8stesting.PatchAction); ok {
			patches = append(patches, p)
		}
	}
	if len(patches) != 1 {
		t.Fatalf("got %d patch actions, want 1", len(patches))
	}
	if sub := patches[0].GetSubresource(); sub != "scale" {
		t.Errorf("patched subresource %q, want \"scale\"", sub)
	}
	if body := string(patches[0].GetPatch()); body != `{"spec":{"replicas":3}}` {
		t.Errorf("patch body = %s", body)
	}

	st, err := c.Status(context.Background(), "default", "demo-canary", "demo")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Desired != 3 {
		t.Errorf("desired replicas after patch = %d, want 3", st.Desired)
	}
}

func TestSetImage(t *testing.T) {
	tests := []struct {
		name       string
		containers []corev1.Container
		want       string
		wantErr    bool
	}{
		{name: "single container", containers: []corev1.Container{{Name: "web", Image: "demo:v1"}}, want: "web"},
		{name: "picks container named after the app", containers: []corev1.Container{{Name: "sidecar", Image: "proxy:1"}, {Name: "demo", Image: "demo:v1"}}, want: "demo"},
		{name: "several containers, none named after the app", containers: []corev1.Container{{Name: "a", Image: "x"}, {Name: "b", Image: "y"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewClientset(deployment("demo-canary", 0, tt.containers...))
			c := NewFromClientset(cs)

			err := c.SetImage(context.Background(), "default", "demo-canary", "demo", "demo:v2")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			d, _ := cs.AppsV1().Deployments("default").Get(context.Background(), "demo-canary", metav1.GetOptions{})
			for _, ctr := range d.Spec.Template.Spec.Containers {
				switch {
				case ctr.Name == tt.want && ctr.Image != "demo:v2":
					t.Errorf("container %s image = %s, want demo:v2", ctr.Name, ctr.Image)
				case ctr.Name != tt.want && ctr.Image == "demo:v2":
					t.Errorf("container %s was changed too", ctr.Name)
				}
			}
			if *d.Spec.Replicas != 0 || d.Labels["app"] != "demo" {
				t.Errorf("image patch changed other fields: replicas=%d labels=%v", *d.Spec.Replicas, d.Labels)
			}
		})
	}
}

func TestErrorsAreMapped(t *testing.T) {
	cs := fake.NewClientset()
	c := NewFromClientset(cs)
	if _, err := c.Status(context.Background(), "default", "missing", "demo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing deployment err = %v, want ErrNotFound", err)
	}

	cs.PrependReactor("patch", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "demo-canary", errors.New("rbac"))
	})
	if err := c.SetReplicas(context.Background(), "default", "demo-canary", 1); !errors.Is(err, ErrForbidden) {
		t.Errorf("forbidden patch err = %v, want ErrForbidden", err)
	}
}

func TestReadyAt(t *testing.T) {
	ready := DeploymentStatus{Desired: 5, Replicas: 5, Ready: 5, Updated: 5, Available: 5, Generation: 2, ObservedGeneration: 2}
	tests := []struct {
		name    string
		mutate  func(*DeploymentStatus)
		desired int32
		want    bool
	}{
		{name: "all at desired", mutate: func(*DeploymentStatus) {}, desired: 5, want: true},
		{name: "asking for a different count", mutate: func(*DeploymentStatus) {}, desired: 6, want: false},
		{name: "controller has not seen the latest spec", mutate: func(s *DeploymentStatus) { s.Generation = 3 }, desired: 5, want: false},
		{name: "old pods still running", mutate: func(s *DeploymentStatus) { s.Replicas = 7 }, desired: 5, want: false},
		{name: "not all pods on the new template", mutate: func(s *DeploymentStatus) { s.Updated = 3 }, desired: 5, want: false},
		{name: "not all ready", mutate: func(s *DeploymentStatus) { s.Ready = 4 }, desired: 5, want: false},
		{name: "not all available", mutate: func(s *DeploymentStatus) { s.Available = 4 }, desired: 5, want: false},
		{name: "scaled to zero", mutate: func(s *DeploymentStatus) { *s = DeploymentStatus{Generation: 4, ObservedGeneration: 4} }, desired: 0, want: true},
		{name: "scaling to zero, pods remain", mutate: func(s *DeploymentStatus) { s.Desired = 0; s.Updated = 0; s.Ready = 0; s.Available = 0 }, desired: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ready
			tt.mutate(&s)
			if got := s.ReadyAt(tt.desired); got != tt.want {
				t.Errorf("ReadyAt(%d) = %v, want %v for %+v", tt.desired, got, tt.want, s)
			}
		})
	}
}

func TestNames(t *testing.T) {
	if StableName("demo") != "demo-stable" || CanaryName("demo") != "demo-canary" {
		t.Errorf("names = %s, %s", StableName("demo"), CanaryName("demo"))
	}
}

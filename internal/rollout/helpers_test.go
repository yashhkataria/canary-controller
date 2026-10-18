package rollout

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDurationJSON(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: `"20s"`, want: 20 * time.Second},
		{in: `"1m30s"`, want: 90 * time.Second},
		{in: `"500ms"`, want: 500 * time.Millisecond},
		{in: `"20"`, wantErr: true},
		{in: `"soon"`, wantErr: true},
		{in: `20`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.in), &d)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && d.Std() != tt.want {
				t.Errorf("got %v, want %v", d.Std(), tt.want)
			}
		})
	}
}

func TestDurationMarshal(t *testing.T) {
	b, err := json.Marshal(DefaultAnalysis())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["interval"] != "20s" || got["maxWait"] != "5m0s" {
		t.Errorf("interval = %v, maxWait = %v; want \"20s\", \"5m0s\"", got["interval"], got["maxWait"])
	}
}

func TestPhaseTerminal(t *testing.T) {
	tests := []struct {
		phase Phase
		want  bool
	}{
		{PhasePending, false},
		{PhaseProgressing, false},
		{PhaseAnalyzing, false},
		{PhasePromoting, false},
		{PhaseRollingBack, false},
		{PhaseSucceeded, true},
		{PhaseRolledBack, true},
	}
	for _, tt := range tests {
		if got := tt.phase.Terminal(); got != tt.want {
			t.Errorf("%s.Terminal() = %v, want %v", tt.phase, got, tt.want)
		}
	}
}

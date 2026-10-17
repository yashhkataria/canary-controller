package demoapp

import (
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: Config{Version: DefaultVersion, Track: DefaultTrack},
		},
		{
			name: "all set",
			env:  map[string]string{"VERSION": "v2", "TRACK": "canary", "ERROR_RATE": "0.2", "EXTRA_LATENCY_MS": "800"},
			want: Config{Version: "v2", Track: "canary", ErrorRate: 0.2, ExtraLatency: 800 * time.Millisecond},
		},
		{name: "error rate 0", env: map[string]string{"ERROR_RATE": "0"}, want: Config{Version: DefaultVersion, Track: DefaultTrack}},
		{name: "error rate 1", env: map[string]string{"ERROR_RATE": "1"}, want: Config{Version: DefaultVersion, Track: DefaultTrack, ErrorRate: 1}},
		{name: "error rate above 1", env: map[string]string{"ERROR_RATE": "1.5"}, wantErr: true},
		{name: "error rate negative", env: map[string]string{"ERROR_RATE": "-0.1"}, wantErr: true},
		{name: "error rate NaN", env: map[string]string{"ERROR_RATE": "NaN"}, wantErr: true},
		{name: "error rate not a number", env: map[string]string{"ERROR_RATE": "20%"}, wantErr: true},
		{name: "latency negative", env: map[string]string{"EXTRA_LATENCY_MS": "-1"}, wantErr: true},
		{name: "latency with unit", env: map[string]string{"EXTRA_LATENCY_MS": "800ms"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConfigFromEnv(func(k string) string { return tt.env[k] })
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

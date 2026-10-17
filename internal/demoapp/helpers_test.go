package demoapp

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func steppedRand() func() float64 {
	i := 0
	return func() float64 {
		v := float64(i%10) / 10
		i++
		return v
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func family(t *testing.T, app *App, name string) *dto.MetricFamily {
	t.Helper()
	fams, err := app.reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range fams {
		if f.GetName() == name {
			return f
		}
	}
	t.Fatalf("metric %s not found", name)
	return nil
}

func assertLabelNames(t *testing.T, f *dto.MetricFamily, want ...string) {
	t.Helper()
	for _, m := range f.GetMetric() {
		var got []string
		for _, l := range m.GetLabel() {
			got = append(got, l.GetName())
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s labels = %v, want %v", f.GetName(), got, want)
		}
	}
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

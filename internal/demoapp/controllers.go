package demoapp

import (
	"net/http"
	"time"
)

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !sleepCtx(r.Context(), a.cfg.ExtraLatency) {
		return
	}

	code := http.StatusOK
	body := rootResponse{Version: a.cfg.Version, Track: a.cfg.Track}
	if a.rand() < a.cfg.ErrorRate {
		code = http.StatusInternalServerError
		body.Error = injectedErrorMessage
	}

	writeJSON(w, code, body)
	a.metrics.observe(a.cfg.Track, code, time.Since(start))
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

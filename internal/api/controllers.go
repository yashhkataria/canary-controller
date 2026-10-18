package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/yashhkataria/canary-controller/internal/logging"
	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

func (s *Server) createRollout(w http.ResponseWriter, r *http.Request) {
	var req CreateRolloutRequest
	if status, err := decodeJSON(w, r, &req); err != nil {
		writeError(w, status, err.Error(), nil)
		return
	}
	if fields := validateCreate(req); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, "invalid rollout", fields)
		return
	}

	created, err := s.store.CreateRollout(r.Context(), rollout.Rollout{
		Namespace:   req.Namespace,
		App:         req.App,
		CanaryImage: req.Image,
		Steps:       req.Steps,
		Phase:       rollout.PhasePending,
		Analysis:    req.Analysis.merged(),
	})
	if errors.Is(err, store.ErrActiveRolloutExists) {
		writeError(w, http.StatusConflict, fmt.Sprintf("an active rollout already exists for %s/%s", req.Namespace, req.App), nil)
		return
	}
	if err != nil {
		s.internalError(w, r, "create rollout", err)
		return
	}

	log := logging.From(r.Context()).With("rollout_id", created.ID.Hex())
	if _, err := s.store.AppendEvent(r.Context(), rollout.Event{
		RolloutID: created.ID,
		At:        created.CreatedAt,
		Type:      rollout.EventCreated,
		Message:   fmt.Sprintf("rollout of %s to %s/%s created", created.CanaryImage, created.Namespace, created.App),
	}); err != nil {
		log.Warn("write Created event", "err", err)
	}
	log.Info("rollout created", "namespace", created.Namespace, "app", created.App, "image", created.CanaryImage)

	w.Header().Set("Location", "/v1/rollouts/"+created.ID.Hex())
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) getRollout(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ro, err := s.store.GetRollout(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rollout not found", nil)
		return
	}
	if err != nil {
		s.internalError(w, r, "get rollout", err)
		return
	}
	writeJSON(w, http.StatusOK, ro)
}

func (s *Server) listRollouts(w http.ResponseWriter, r *http.Request) {
	filter, fields := parseListFilter(r)
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, "invalid query", fields)
		return
	}
	items, err := s.store.ListRollouts(r.Context(), filter)
	if err != nil {
		s.internalError(w, r, "list rollouts", err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[rollout.Rollout]{Items: items})
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := s.store.GetRollout(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rollout not found", nil)
		return
	} else if err != nil {
		s.internalError(w, r, "get rollout", err)
		return
	}
	items, err := s.store.ListEvents(r.Context(), id)
	if err != nil {
		s.internalError(w, r, "list events", err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[rollout.Event]{Items: items})
}

func (s *Server) notImplemented(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented, action+" is not implemented yet", nil)
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(headerContentType, "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		logging.From(r.Context()).Warn("not ready: mongodb ping failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "mongodb unavailable", nil)
		return
	}
	w.Header().Set(headerContentType, "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/logging"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) (int, error) {
	mt, _, err := mime.ParseMediaType(r.Header.Get(headerContentType))
	if err != nil || mt != contentTypeJSON {
		return http.StatusUnsupportedMediaType, fmt.Errorf("Content-Type must be %s", contentTypeJSON)
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return http.StatusRequestEntityTooLarge, fmt.Errorf("request body must be at most %d bytes", maxBodyBytes)
		}
		return http.StatusBadRequest, fmt.Errorf("invalid JSON body: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return http.StatusBadRequest, errors.New("invalid JSON body: must contain a single JSON object")
	}
	return 0, nil
}

func parseID(w http.ResponseWriter, r *http.Request) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rollout id", nil)
		return bson.ObjectID{}, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string, fields map[string]string) {
	writeJSON(w, code, errorResponse{Error: msg, Fields: fields})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	logging.From(r.Context()).Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error", nil)
}

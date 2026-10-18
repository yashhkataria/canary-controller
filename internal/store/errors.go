package store

import "errors"

var (
	ErrNotFound            = errors.New("rollout not found")
	ErrActiveRolloutExists = errors.New("an active rollout already exists for this app")
	ErrVersionConflict     = errors.New("rollout was changed by someone else")
)

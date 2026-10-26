package store

import "github.com/yashhkataria/canary-controller/internal/rollout"

var (
	ErrNotFound            = rollout.ErrNotFound
	ErrActiveRolloutExists = rollout.ErrActiveRolloutExists
	ErrVersionConflict     = rollout.ErrVersionConflict
)

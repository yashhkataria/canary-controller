package store

import "time"

const (
	collectionRollouts = "rollouts"
	collectionEvents   = "events"
)

const (
	IndexOneActivePerApp = "one_active_per_app"
	IndexActiveUpdated   = "active_updated"
	IndexAppHistory      = "app_history"
	IndexEventsByRollout = "events_by_rollout"
)

const (
	connectTimeout         = 10 * time.Second
	serverSelectionTimeout = 5 * time.Second
	opTimeout              = 5 * time.Second
)

const (
	DefaultListLimit = 20
	MaxListLimit     = 100
)

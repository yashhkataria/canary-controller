package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func (s *Store) AppendEvent(ctx context.Context, e rollout.Event) (rollout.Event, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	e.ID = bson.NewObjectID()
	if e.At.IsZero() {
		e.At = now()
	}
	if _, err := s.events.InsertOne(ctx, e); err != nil {
		return rollout.Event{}, fmt.Errorf("insert event: %w", err)
	}
	return e, nil
}

func (s *Store) ListEvents(ctx context.Context, rolloutID bson.ObjectID) ([]rollout.Event, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "at", Value: 1}, {Key: "_id", Value: 1}})
	cur, err := s.events.Find(ctx, bson.D{{Key: "rolloutId", Value: rolloutID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := []rollout.Event{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	return out, nil
}

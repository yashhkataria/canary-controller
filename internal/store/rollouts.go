package store

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func (s *Store) CreateRollout(ctx context.Context, r rollout.Rollout) (rollout.Rollout, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	now := now()
	r.ID = bson.NewObjectID()
	r.Version = 1
	r.Active = true
	r.CreatedAt = now
	r.UpdatedAt = now

	if _, err := s.rollouts.InsertOne(ctx, r); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return rollout.Rollout{}, ErrActiveRolloutExists
		}
		return rollout.Rollout{}, fmt.Errorf("insert rollout: %w", err)
	}
	return r, nil
}

func (s *Store) GetRollout(ctx context.Context, id bson.ObjectID) (rollout.Rollout, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var r rollout.Rollout
	err := s.rollouts.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&r)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return rollout.Rollout{}, ErrNotFound
	}
	if err != nil {
		return rollout.Rollout{}, fmt.Errorf("get rollout: %w", err)
	}
	return r, nil
}

func (s *Store) ListRollouts(ctx context.Context, f ListFilter) ([]rollout.Rollout, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	filter := bson.D{{Key: "app", Value: f.App}}
	if f.Namespace != "" {
		filter = append(filter, bson.E{Key: "namespace", Value: f.Namespace})
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(clampLimit(f.Limit)))

	cur, err := s.rollouts.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list rollouts: %w", err)
	}
	out := []rollout.Rollout{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("decode rollouts: %w", err)
	}
	return out, nil
}

func (s *Store) ListActive(ctx context.Context) ([]rollout.Rollout, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "updatedAt", Value: 1}}).
		SetLimit(MaxListLimit)
	cur, err := s.rollouts.Find(ctx, bson.D{{Key: "active", Value: true}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list active rollouts: %w", err)
	}
	out := []rollout.Rollout{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("decode active rollouts: %w", err)
	}
	return out, nil
}

func (s *Store) UpdateRollout(ctx context.Context, r rollout.Rollout) (rollout.Rollout, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	filter := bson.D{{Key: "_id", Value: r.ID}, {Key: "version", Value: r.Version}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "stableImage", Value: r.StableImage},
			{Key: "currentStep", Value: r.CurrentStep},
			{Key: "phase", Value: r.Phase},
			{Key: "lastCheck", Value: r.LastCheck},
			{Key: "stepChecks", Value: r.StepChecks},
			{Key: "active", Value: r.Active},
			{Key: "owner", Value: r.Owner},
			{Key: "reason", Value: r.Reason},
			{Key: "finishedAt", Value: r.FinishedAt},
			{Key: "stepStartedAt", Value: r.StepStartedAt},
			{Key: "waitingSince", Value: r.WaitingSince},
			{Key: "updatedAt", Value: now()},
		}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var out rollout.Rollout
	err := s.rollouts.FindOneAndUpdate(ctx, filter, update, opts).Decode(&out)
	switch {
	case err == nil:
		return out, nil
	case mongo.IsDuplicateKeyError(err):
		return rollout.Rollout{}, ErrActiveRolloutExists
	case errors.Is(err, mongo.ErrNoDocuments):
		return rollout.Rollout{}, s.missingOrConflict(ctx, r.ID)
	default:
		return rollout.Rollout{}, fmt.Errorf("update rollout: %w", err)
	}
}

func (s *Store) missingOrConflict(ctx context.Context, id bson.ObjectID) error {
	n, err := s.rollouts.CountDocuments(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("check rollout exists: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return ErrVersionConflict
}

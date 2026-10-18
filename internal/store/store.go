package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func New(ctx context.Context, uri, database string) (*Store, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(serverSelectionTimeout))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}
	s := &Store{
		client:   client,
		rollouts: client.Database(database).Collection(collectionRollouts),
		events:   client.Database(database).Collection(collectionEvents),
	}
	if err := s.Ping(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}
	return s, nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return s.client.Ping(ctx, readpref.Primary())
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := s.rollouts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "namespace", Value: 1}, {Key: "app", Value: 1}},
			Options: options.Index().
				SetName(IndexOneActivePerApp).
				SetUnique(true).
				SetPartialFilterExpression(bson.D{{Key: "active", Value: true}}),
		},
		{
			Keys:    bson.D{{Key: "active", Value: 1}, {Key: "updatedAt", Value: 1}},
			Options: options.Index().SetName(IndexActiveUpdated),
		},
		{
			Keys:    bson.D{{Key: "namespace", Value: 1}, {Key: "app", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName(IndexAppHistory),
		},
	})
	if err != nil {
		return fmt.Errorf("create rollout indexes: %w", err)
	}

	_, err = s.events.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "rolloutId", Value: 1}, {Key: "at", Value: 1}},
		Options: options.Index().SetName(IndexEventsByRollout),
	})
	if err != nil {
		return fmt.Errorf("create event indexes: %w", err)
	}
	return nil
}

package store

import "go.mongodb.org/mongo-driver/v2/mongo"

type Store struct {
	client   *mongo.Client
	rollouts *mongo.Collection
	events   *mongo.Collection
}

type ListFilter struct {
	Namespace string
	App       string
	Limit     int
}

package store_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

const mongoImage = "mongo:9.0.4"

var (
	mongoURI string
	dbSeq    atomic.Int64
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	container, err := mongodb.Run(ctx, mongoImage)
	if err != nil {
		log.Fatalf("start mongodb container: %v", err)
	}
	mongoURI, err = container.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("mongodb connection string: %v", err)
	}

	code := m.Run()
	if err := testcontainers.TerminateContainer(container); err != nil {
		log.Printf("terminate mongodb container: %v", err)
	}
	os.Exit(code)
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, _ := newStoreWithDB(t)
	return s
}

func newStoreWithDB(t *testing.T) (*store.Store, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: needs Docker")
	}
	ctx := context.Background()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db := fmt.Sprintf("t%d_%s", dbSeq.Add(1), name)
	if len(db) > 60 {
		db = db[:60]
	}

	s, err := store.New(ctx, mongoURI, db)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	if err := s.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s, db
}

func indexNames(t *testing.T, db string) map[string][]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("connect to mongodb: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	out := map[string][]string{}
	for _, coll := range []string{"rollouts", "events"} {
		specs, err := client.Database(db).Collection(coll).Indexes().ListSpecifications(ctx)
		if err != nil {
			t.Fatalf("list indexes of %s: %v", coll, err)
		}
		for _, spec := range specs {
			out[coll] = append(out[coll], spec.Name)
		}
	}
	return out
}

func newRollout(namespace, app string) rollout.Rollout {
	return rollout.Rollout{
		Namespace:   namespace,
		App:         app,
		CanaryImage: "demo:v2",
		Steps:       []int{10, 50, 100},
		Phase:       rollout.PhasePending,
		Analysis:    rollout.DefaultAnalysis(),
	}
}

func mustCreate(t *testing.T, s *store.Store, r rollout.Rollout) rollout.Rollout {
	t.Helper()
	created, err := s.CreateRollout(context.Background(), r)
	if err != nil {
		t.Fatalf("CreateRollout: %v", err)
	}
	return created
}

func finish(t *testing.T, s *store.Store, r rollout.Rollout) rollout.Rollout {
	t.Helper()
	r.Phase = rollout.PhaseRolledBack
	r.Active = false
	out, err := s.UpdateRollout(context.Background(), r)
	if err != nil {
		t.Fatalf("UpdateRollout: %v", err)
	}
	return out
}

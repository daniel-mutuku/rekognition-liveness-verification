package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

// testStoreContract runs against both Memory and DynamoDB, so the fake can't drift.
func testStoreContract(t *testing.T, newStore func(t *testing.T) verify.Store) {
	ctx := context.Background()

	// A real table is shared across subtests and runs.
	run := verify.NewID()[:8]
	id := func(t *testing.T) string { return run + "/" + t.Name() }

	created := time.Date(2026, 10, 28, 14, 32, 0, 0, time.UTC)
	sample := func(id string) verify.Verification {
		return verify.Verification{
			ID:        id,
			Status:    verify.StatusCreated,
			CreatedAt: created,
			UpdatedAt: created,
			ExpiresAt: created.Add(7 * 24 * time.Hour),
		}
	}

	t.Run("create then get returns the same record", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		v.LivenessSessionID = "sess-1"
		v.LivenessAttempts = 2
		v.PhotoETag = `"9b2cf535f27731c974343645a3985328"`
		v.ReferenceKey = "liveness/a1/sess-1/reference.jpg"
		v.Scores = &verify.Scores{Liveness: 93.5, Similarity: 86.25}
		v.Reasons = []verify.Reason{verify.ReasonMatchUncertain}

		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if v.Version != 1 {
			t.Errorf("Version after Create = %d, want 1", v.Version)
		}
		got, err := s.Get(ctx, id(t))
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !reflect.DeepEqual(got, v) {
			t.Errorf("Get =\n %+v\nwant\n %+v", got, v)
		}
	})

	t.Run("create with a taken id", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}
		again := sample(id(t))
		if err := s.Create(ctx, &again); !errors.Is(err, verify.ErrExists) {
			t.Errorf("second Create err = %v, want ErrExists", err)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		if _, err := newStore(t).Get(ctx, id(t)+"-missing"); !errors.Is(err, verify.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("update saves and bumps the version", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}
		v.Status = verify.StatusPhotoOK
		if err := s.Update(ctx, &v); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if v.Version != 2 {
			t.Errorf("Version after Update = %d, want 2", v.Version)
		}
		got, _ := s.Get(ctx, id(t))
		if got.Status != verify.StatusPhotoOK || got.Version != 2 {
			t.Errorf("stored status %q version %d", got.Status, got.Version)
		}
	})

	t.Run("update from a stale copy conflicts", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}
		// Two requests read the same version...
		first, _ := s.Get(ctx, id(t))
		second, _ := s.Get(ctx, id(t))
		// ...the first saves, so the second's copy is now stale.
		if err := s.Update(ctx, &first); err != nil {
			t.Fatalf("first Update: %v", err)
		}
		if err := s.Update(ctx, &second); !errors.Is(err, verify.ErrConflict) {
			t.Errorf("stale Update err = %v, want ErrConflict", err)
		}
	})

	t.Run("update missing", func(t *testing.T) {
		v := sample(id(t) + "-missing")
		v.Version = 1
		if err := newStore(t).Update(ctx, &v); !errors.Is(err, verify.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("changing a returned record doesn't change the stored one", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		v.Reasons = []verify.Reason{verify.ReasonFaceMismatch}
		v.Scores = &verify.Scores{Liveness: 98, Similarity: 12}
		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, _ := s.Get(ctx, id(t))
		got.Reasons[0] = verify.ReasonLivenessFailed
		got.Scores.Similarity = 99
		v.Reasons[0] = verify.ReasonLivenessFailed // the caller's own copy, too

		again, _ := s.Get(ctx, id(t))
		if again.Reasons[0] != verify.ReasonFaceMismatch || again.Scores.Similarity != 12 {
			t.Errorf("stored record was changed through a copy: %+v", again)
		}
	})

	t.Run("concurrent updates: exactly one wins", func(t *testing.T) {
		s := newStore(t)
		v := sample(id(t))
		if err := s.Create(ctx, &v); err != nil {
			t.Fatalf("Create: %v", err)
		}

		const n = 10
		var wg sync.WaitGroup
		errs := make(chan error, n)
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := v // every goroutine starts from version 1
				errs <- s.Update(ctx, &c)
			}()
		}
		wg.Wait()
		close(errs)

		wins := 0
		for err := range errs {
			switch {
			case err == nil:
				wins++
			case !errors.Is(err, verify.ErrConflict):
				t.Errorf("unexpected error: %v", err)
			}
		}
		if wins != 1 {
			t.Errorf("%d updates succeeded, want exactly 1", wins)
		}
	})
}

func TestMemory(t *testing.T) {
	testStoreContract(t, func(*testing.T) verify.Store { return NewMemory() })
}

// TestDynamo is opt-in (make test-aws). Items it writes expire via TTL.
func TestDynamo(t *testing.T) {
	table := os.Getenv("DYNAMO_TEST_TABLE")
	if table == "" {
		t.Skip("set DYNAMO_TEST_TABLE (and AWS_PROFILE) to run against DynamoDB")
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	testStoreContract(t, func(*testing.T) verify.Store { return NewDynamo(cfg, table) })
}

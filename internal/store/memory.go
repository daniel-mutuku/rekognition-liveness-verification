// Package store implements verify.Store with DynamoDB and an in-memory fake.
package store

import (
	"context"
	"slices"
	"sync"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type Memory struct {
	mu    sync.Mutex
	items map[string]verify.Verification
}

var _ verify.Store = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{items: make(map[string]verify.Verification)}
}

func (m *Memory) Create(_ context.Context, v *verify.Verification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.items[v.ID]; ok {
		return verify.ErrExists
	}
	v.Version = 1
	m.items[v.ID] = clone(*v)
	return nil
}

func (m *Memory) Get(_ context.Context, id string) (verify.Verification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.items[id]
	if !ok {
		return verify.Verification{}, verify.ErrNotFound
	}
	return clone(v), nil
}

func (m *Memory) Update(_ context.Context, v *verify.Verification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.items[v.ID]
	if !ok {
		return verify.ErrNotFound
	}
	if stored.Version != v.Version {
		return verify.ErrConflict
	}
	v.Version++
	m.items[v.ID] = clone(*v)
	return nil
}

// clone deep-copies Reasons and Scores so callers can't mutate stored records.
func clone(v verify.Verification) verify.Verification {
	v.Reasons = slices.Clone(v.Reasons)
	if v.Scores != nil {
		s := *v.Scores
		v.Scores = &s
	}
	return v
}

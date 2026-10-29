package verify

import (
	"context"
	"errors"
)

// Store persists verifications.
type Store interface {
	// Create sets v.Version to 1, or returns ErrExists.
	Create(ctx context.Context, v *Verification) error
	Get(ctx context.Context, id string) (Verification, error)
	// Update saves v only if the stored version equals v.Version, then
	// increments it. Otherwise ErrConflict (or ErrNotFound).
	Update(ctx context.Context, v *Verification) error
}

var (
	ErrNotFound          = errors.New("verification not found")
	ErrExists            = errors.New("verification already exists")
	ErrConflict          = errors.New("verification was changed by another request")
	ErrInvalidTransition = errors.New("invalid status change")
)

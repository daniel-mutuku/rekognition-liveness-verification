package verify

import (
	"fmt"
	"slices"
	"time"
)

// Verification is one attempt: a photo, up to MaxLivenessAttempts liveness
// sessions, then a decision. S3 keys are derived from ID, never stored.
type Verification struct {
	ID        string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time

	// The only session whose result is accepted; a retry replaces it.
	LivenessSessionID string
	LivenessAttempts  int

	// The upload URL outlives the photo check, so the decision re-checks the ETag.
	PhotoETag    string
	ReferenceKey string

	Scores  *Scores
	Reasons []Reason

	// Optimistic locking: see Store.Update.
	Version int
}

var transitions = map[Status][]Status{
	StatusCreated: {StatusPhotoOK},
	StatusPhotoOK: {StatusLivenessStarted},
	// liveness_started → liveness_started is a retry with a new session.
	StatusLivenessStarted: {StatusLivenessStarted, StatusApproved, StatusReview, StatusRejected},
}

func (s Status) CanMoveTo(to Status) bool {
	return slices.Contains(transitions[s], to)
}

func (v Verification) Expired(now time.Time) bool {
	return !now.Before(v.ExpiresAt)
}

func (v *Verification) moveTo(to Status, now time.Time) error {
	if !v.Status.CanMoveTo(to) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, v.Status, to)
	}
	v.Status = to
	v.UpdatedAt = now
	return nil
}

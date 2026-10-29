package verify

import (
	"errors"
	"testing"
	"time"
)

func TestCanMoveTo(t *testing.T) {
	tests := []struct {
		from, to Status
		want     bool
	}{
		{StatusCreated, StatusPhotoOK, true},
		{StatusPhotoOK, StatusLivenessStarted, true},
		{StatusLivenessStarted, StatusLivenessStarted, true}, // liveness retry
		{StatusLivenessStarted, StatusApproved, true},
		{StatusLivenessStarted, StatusReview, true},
		{StatusLivenessStarted, StatusRejected, true},

		{StatusCreated, StatusLivenessStarted, false}, // skipping the photo
		{StatusCreated, StatusApproved, false},
		{StatusPhotoOK, StatusApproved, false}, // skipping liveness
		{StatusPhotoOK, StatusCreated, false},  // going backwards
		{StatusApproved, StatusRejected, false},
		{StatusRejected, StatusApproved, false},
		{StatusReview, StatusLivenessStarted, false},
		{Status("bogus"), StatusPhotoOK, false},
	}
	for _, tt := range tests {
		if got := tt.from.CanMoveTo(tt.to); got != tt.want {
			t.Errorf("%s.CanMoveTo(%s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestMoveTo(t *testing.T) {
	now := time.Date(2026, 10, 28, 14, 32, 0, 0, time.UTC)

	t.Run("allowed", func(t *testing.T) {
		v := Verification{Status: StatusCreated}
		if err := v.moveTo(StatusPhotoOK, now); err != nil {
			t.Fatalf("moveTo: %v", err)
		}
		if v.Status != StatusPhotoOK || !v.UpdatedAt.Equal(now) {
			t.Errorf("got status %q updated %v", v.Status, v.UpdatedAt)
		}
	})

	t.Run("not allowed leaves it unchanged", func(t *testing.T) {
		v := Verification{Status: StatusApproved}
		err := v.moveTo(StatusRejected, now)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
		if v.Status != StatusApproved || !v.UpdatedAt.IsZero() {
			t.Errorf("verification changed: %+v", v)
		}
	})
}

func TestExpired(t *testing.T) {
	exp := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	v := Verification{ExpiresAt: exp}

	if v.Expired(exp.Add(-time.Second)) {
		t.Error("expired one second before ExpiresAt")
	}
	if !v.Expired(exp) {
		t.Error("not expired exactly at ExpiresAt")
	}
}

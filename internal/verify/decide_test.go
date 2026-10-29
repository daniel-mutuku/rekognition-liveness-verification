package verify

import (
	"slices"
	"testing"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name        string
		liveness    float64
		similarity  float64
		wantStatus  Status
		wantReasons []Reason // nil = no reasons
	}{
		{"clear pass", 99.2, 98.6, StatusApproved, nil},
		{"exactly at approve thresholds", 90, 95, StatusApproved, nil},

		{"liveness just under approve", 89.99, 99, StatusReview, []Reason{ReasonLivenessUncertain}},
		{"similarity just under approve", 99, 94.99, StatusReview, []Reason{ReasonMatchUncertain}},
		{"exactly at review thresholds", 70, 80, StatusReview,
			[]Reason{ReasonLivenessUncertain, ReasonMatchUncertain}},

		{"liveness just under review", 69.99, 99, StatusRejected, []Reason{ReasonLivenessFailed}},
		{"similarity just under review", 99, 79.99, StatusRejected, []Reason{ReasonFaceMismatch}},
		{"someone else's photo", 98.1, 12.7, StatusRejected, []Reason{ReasonFaceMismatch}},
		{"photo held up to camera", 4.3, 97, StatusRejected, []Reason{ReasonLivenessFailed}},
		{"both fail", 10, 10, StatusRejected, []Reason{ReasonLivenessFailed, ReasonFaceMismatch}},

		// The worse outcome wins, whichever score it comes from.
		{"rejected liveness beats review match", 50, 85, StatusRejected,
			[]Reason{ReasonLivenessFailed, ReasonMatchUncertain}},
		{"review liveness then rejected match", 80, 50, StatusRejected,
			[]Reason{ReasonLivenessUncertain, ReasonFaceMismatch}},
		{"zero scores", 0, 0, StatusRejected, []Reason{ReasonLivenessFailed, ReasonFaceMismatch}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultThresholds.Decide(Scores{Liveness: tt.liveness, Similarity: tt.similarity})

			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestDecideIsFinal(t *testing.T) {
	// Whatever the scores, Decide only ever returns a final status.
	for _, l := range []float64{0, 70, 90, 100} {
		for _, s := range []float64{0, 80, 95, 100} {
			if d := DefaultThresholds.Decide(Scores{l, s}); !d.Status.Final() {
				t.Errorf("Decide(%v, %v) = %q, not a final status", l, s, d.Status)
			}
		}
	}
}

func TestStatusFinal(t *testing.T) {
	for s, want := range map[Status]bool{
		StatusCreated: false, StatusPhotoOK: false, StatusLivenessStarted: false,
		StatusApproved: true, StatusReview: true, StatusRejected: true,
	} {
		if got := s.Final(); got != want {
			t.Errorf("%q.Final() = %v, want %v", s, got, want)
		}
	}
}

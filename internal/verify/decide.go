// Package verify holds the verification flow and decision rules. AWS sits
// behind the Store, Media and Faces interfaces.
package verify

// Status values are stored and returned by the API; don't rename them.
type Status string

const (
	StatusCreated         Status = "created"
	StatusPhotoOK         Status = "photo_ok"
	StatusLivenessStarted Status = "liveness_started"
	StatusApproved        Status = "approved"
	StatusReview          Status = "review"
	StatusRejected        Status = "rejected"
)

func (s Status) Final() bool {
	return s == StatusApproved || s == StatusReview || s == StatusRejected
}

// Reason codes are part of the API contract; the web app maps them to copy.
type Reason string

const (
	ReasonLivenessFailed    Reason = "liveness_failed"
	ReasonLivenessUncertain Reason = "liveness_uncertain"
	ReasonFaceMismatch      Reason = "face_mismatch"
	ReasonMatchUncertain    Reason = "match_uncertain"

	ReasonLivenessSessionFailed Reason = "liveness_session_failed"
	ReasonNoFaceInReference     Reason = "no_face_in_reference"
	ReasonPhotoChanged          Reason = "photo_changed"
)

// Scores are Rekognition's 0–100 confidence values.
type Scores struct {
	Liveness   float64
	Similarity float64
}

// Thresholds are inclusive minimums. They're returned by the API so the UI
// can draw the pass marks.
type Thresholds struct {
	ApproveLiveness   float64 `json:"approveLiveness"`
	ApproveSimilarity float64 `json:"approveSimilarity"`
	ReviewLiveness    float64 `json:"reviewLiveness"`
	ReviewSimilarity  float64 `json:"reviewSimilarity"`
}

var DefaultThresholds = Thresholds{
	ApproveLiveness:   90,
	ApproveSimilarity: 95,
	ReviewLiveness:    70,
	ReviewSimilarity:  80,
}

type Decision struct {
	Status  Status
	Reasons []Reason
}

// Decide judges each score on its own and the worse outcome wins: a perfect
// match can't make up for weak liveness, or the other way round.
func (t Thresholds) Decide(s Scores) Decision {
	var reasons []Reason
	status := StatusApproved

	worsen := func(to Status, why Reason) {
		reasons = append(reasons, why)
		if status != StatusRejected {
			status = to
		}
	}

	switch {
	case s.Liveness < t.ReviewLiveness:
		worsen(StatusRejected, ReasonLivenessFailed)
	case s.Liveness < t.ApproveLiveness:
		worsen(StatusReview, ReasonLivenessUncertain)
	}

	switch {
	case s.Similarity < t.ReviewSimilarity:
		worsen(StatusRejected, ReasonFaceMismatch)
	case s.Similarity < t.ApproveSimilarity:
		worsen(StatusReview, ReasonMatchUncertain)
	}

	return Decision{Status: status, Reasons: reasons}
}

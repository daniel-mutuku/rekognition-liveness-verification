package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type api struct {
	logger *slog.Logger
	svc    *verify.Service
}

func (a *api) withID(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !verify.ValidID(id) {
			writeError(w, http.StatusBadRequest, "invalid_id", "verification id must be a UUID")
			return
		}
		h(w, r, id)
	}
}

type createdJSON struct {
	ID              string            `json:"id"`
	Status          verify.Status     `json:"status"`
	ExpiresAt       time.Time         `json:"expiresAt"`
	UploadURL       string            `json:"uploadUrl"`
	UploadHeaders   map[string]string `json:"uploadHeaders"`
	UploadExpiresAt time.Time         `json:"uploadExpiresAt"`
}

func (a *api) create(w http.ResponseWriter, r *http.Request) {
	c, err := a.svc.Create(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, createdJSON{
		ID:              c.Verification.ID,
		Status:          c.Verification.Status,
		ExpiresAt:       c.Verification.ExpiresAt,
		UploadURL:       c.UploadURL,
		UploadHeaders:   c.UploadHeaders,
		UploadExpiresAt: c.UploadExpiresAt,
	})
}

func (a *api) get(w http.ResponseWriter, r *http.Request, id string) {
	v, err := a.svc.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toVerificationJSON(v))
}

type statusJSON struct {
	ID     string        `json:"id"`
	Status verify.Status `json:"status"`
}

func (a *api) checkPhoto(w http.ResponseWriter, r *http.Request, id string) {
	v, err := a.svc.CheckPhoto(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statusJSON{ID: v.ID, Status: v.Status})
}

type sessionJSON struct {
	SessionID   string `json:"sessionId"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"maxAttempts"`
}

func (a *api) startLiveness(w http.ResponseWriter, r *http.Request, id string) {
	sessionID, v, err := a.svc.StartLiveness(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionJSON{
		SessionID:   sessionID,
		Attempt:     v.LivenessAttempts,
		MaxAttempts: a.svc.MaxLivenessAttempts(),
	})
}

func (a *api) complete(w http.ResponseWriter, r *http.Request, id string) {
	v, err := a.svc.Complete(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toVerificationJSON(v))
}

type verificationJSON struct {
	ID                  string            `json:"id"`
	Status              verify.Status     `json:"status"`
	CreatedAt           time.Time         `json:"createdAt"`
	ExpiresAt           time.Time         `json:"expiresAt"`
	LivenessAttempts    int               `json:"livenessAttempts"`
	MaxLivenessAttempts int               `json:"maxLivenessAttempts"`
	LivenessConfidence  *float64          `json:"livenessConfidence,omitempty"`
	Similarity          *float64          `json:"similarity,omitempty"`
	Reasons             []verify.Reason   `json:"reasons"`
	Thresholds          verify.Thresholds `json:"thresholds"`
	PhotoURL            string            `json:"photoUrl,omitempty"`
	ReferenceURL        string            `json:"referenceUrl,omitempty"`
}

func toVerificationJSON(v verify.View) verificationJSON {
	j := verificationJSON{
		ID:                  v.ID,
		Status:              v.Status,
		CreatedAt:           v.CreatedAt,
		ExpiresAt:           v.ExpiresAt,
		LivenessAttempts:    v.LivenessAttempts,
		MaxLivenessAttempts: v.MaxLivenessAttempts,
		Reasons:             v.Reasons,
		Thresholds:          v.Thresholds,
		PhotoURL:            v.PhotoURL,
		ReferenceURL:        v.ReferenceURL,
	}
	if j.Reasons == nil {
		j.Reasons = []verify.Reason{}
	}
	if v.Scores != nil {
		j.LivenessConfidence = &v.Scores.Liveness
		// Similarity is 0 when no faces were compared; omit it rather than show 0.
		if v.ReferenceKey != "" && !hasReason(v.Reasons, verify.ReasonPhotoChanged, verify.ReasonNoFaceInReference) {
			j.Similarity = &v.Scores.Similarity
		}
	}
	return j
}

func hasReason(reasons []verify.Reason, codes ...verify.Reason) bool {
	return slices.ContainsFunc(reasons, func(r verify.Reason) bool { return slices.Contains(codes, r) })
}

// fail maps service errors to responses. Anything unlisted is logged and
// returned as a generic 500, so AWS error details never reach the client.
func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	var rejected *verify.PhotoRejectedError
	switch {
	case errors.As(err, &rejected):
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{
			Error: "photo didn't pass the checks", Code: "photo_rejected", Reasons: rejected.Reasons,
		})
	case errors.Is(err, verify.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "verification not found")
	case errors.Is(err, verify.ErrExpired):
		writeError(w, http.StatusGone, "expired", "verification has expired")
	case errors.Is(err, verify.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "wrong_step", "that step isn't available for this verification now")
	case errors.Is(err, verify.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "verification was changed by another request; reload it")
	case errors.Is(err, verify.ErrPhotoMissing):
		writeError(w, http.StatusConflict, "photo_missing", "no photo has been uploaded yet")
	case errors.Is(err, verify.ErrTooManyAttempts):
		writeError(w, http.StatusConflict, "too_many_attempts", "no live check attempts left; start a new verification")
	case errors.Is(err, verify.ErrLivenessNotFinished):
		writeError(w, http.StatusConflict, "liveness_not_finished", "the live check hasn't finished yet")
	default:
		a.logger.LogAttrs(r.Context(), slog.LevelError, "request failed",
			slog.String("path", r.URL.Path), slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

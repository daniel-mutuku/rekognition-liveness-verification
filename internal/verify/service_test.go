package verify_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/faces"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/media"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/store"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type env struct {
	svc   *verify.Service
	store *store.Memory
	media *media.Memory
	faces *faces.Fake
	now   time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		store: store.NewMemory(),
		media: media.NewMemory(),
		faces: faces.NewFake(),
		now:   time.Date(2026, 10, 28, 14, 32, 0, 0, time.UTC),
	}
	svc, err := verify.NewService(verify.Deps{
		Store: e.store, Media: e.media, Faces: e.faces,
		Retention: 7 * 24 * time.Hour,
		Now:       func() time.Time { return e.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

var ctx = context.Background()

func (e *env) upload(id string, data string, found ...verify.Face) {
	e.media.Put(verify.PhotoKey(id), []byte(data))
	e.faces.SetPhotoFaces(verify.PhotoKey(id), found...)
}

func (e *env) toLiveness(t *testing.T) (id, session string) {
	t.Helper()
	c, err := e.svc.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id = c.Verification.ID
	e.upload(id, "jpeg bytes", faces.GoodFace)
	if _, err := e.svc.CheckPhoto(ctx, id); err != nil {
		t.Fatalf("CheckPhoto: %v", err)
	}
	session, _, err = e.svc.StartLiveness(ctx, id)
	if err != nil {
		t.Fatalf("StartLiveness: %v", err)
	}
	return id, session
}

func TestHappyPath(t *testing.T) {
	e := newEnv(t)

	c, err := e.svc.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v := c.Verification
	if !verify.ValidID(v.ID) || v.Status != verify.StatusCreated {
		t.Fatalf("created %+v", v)
	}
	if want := e.now.Add(7 * 24 * time.Hour); !v.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", v.ExpiresAt, want)
	}
	if c.UploadURL == "" || c.UploadHeaders["Content-Type"] != "image/jpeg" {
		t.Errorf("upload = %q %v", c.UploadURL, c.UploadHeaders)
	}

	e.upload(v.ID, "jpeg bytes", faces.GoodFace)
	if v, err = e.svc.CheckPhoto(ctx, v.ID); err != nil || v.Status != verify.StatusPhotoOK {
		t.Fatalf("CheckPhoto = %q, %v", v.Status, err)
	}

	session, v, err := e.svc.StartLiveness(ctx, v.ID)
	if err != nil || session == "" || v.LivenessAttempts != 1 {
		t.Fatalf("StartLiveness = %q, attempts %d, %v", session, v.LivenessAttempts, err)
	}

	e.faces.FinishSession(session, verify.LivenessSucceeded, 99.2)
	e.faces.SetMatch(verify.Match{FaceFound: true, Similarity: 98.6})
	view, err := e.svc.Complete(ctx, v.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if view.Status != verify.StatusApproved || view.Reasons != nil {
		t.Errorf("decision = %q %v", view.Status, view.Reasons)
	}
	if *view.Scores != (verify.Scores{Liveness: 99.2, Similarity: 98.6}) {
		t.Errorf("scores = %+v", *view.Scores)
	}
	if view.PhotoURL == "" || view.ReferenceURL == "" {
		t.Errorf("image URLs missing: %q %q", view.PhotoURL, view.ReferenceURL)
	}
	if view.Thresholds != verify.DefaultThresholds || view.MaxLivenessAttempts != 3 {
		t.Errorf("rules = %+v %d", view.Thresholds, view.MaxLivenessAttempts)
	}

	got, err := e.svc.Get(ctx, v.ID)
	if err != nil || got.Status != verify.StatusApproved {
		t.Errorf("Get = %q, %v", got.Status, err)
	}
}

func TestCheckPhoto(t *testing.T) {
	t.Run("not uploaded yet", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		if _, err := e.svc.CheckPhoto(ctx, c.Verification.ID); !errors.Is(err, verify.ErrPhotoMissing) {
			t.Errorf("err = %v, want ErrPhotoMissing", err)
		}
	})

	t.Run("rejected photo keeps the verification in created", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		id := c.Verification.ID
		e.upload(id, "group photo", faces.GoodFace, faces.GoodFace)

		_, err := e.svc.CheckPhoto(ctx, id)
		var rejected *verify.PhotoRejectedError
		if !errors.As(err, &rejected) || !slices.Equal(rejected.Reasons, []verify.Reason{verify.ReasonMultipleFaces}) {
			t.Fatalf("err = %v, want PhotoRejectedError{multiple_faces}", err)
		}
		if v, _ := e.store.Get(ctx, id); v.Status != verify.StatusCreated {
			t.Errorf("status = %q, want created", v.Status)
		}

		// A better photo under the same id is then accepted.
		e.upload(id, "selfie", faces.GoodFace)
		if v, err := e.svc.CheckPhoto(ctx, id); err != nil || v.Status != verify.StatusPhotoOK {
			t.Errorf("second CheckPhoto = %q, %v", v.Status, err)
		}
	})

	t.Run("not an image", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		e.media.Put(verify.PhotoKey(c.Verification.ID), []byte("%PDF-1.7")) // unknown to the fake: unreadable
		_, err := e.svc.CheckPhoto(ctx, c.Verification.ID)
		var rejected *verify.PhotoRejectedError
		if !errors.As(err, &rejected) || rejected.Reasons[0] != verify.ReasonUnreadable {
			t.Errorf("err = %v, want unreadable", err)
		}
	})

	t.Run("too large", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		e.upload(c.Verification.ID, string(make([]byte, 15<<20+1)), faces.GoodFace)
		_, err := e.svc.CheckPhoto(ctx, c.Verification.ID)
		var rejected *verify.PhotoRejectedError
		if !errors.As(err, &rejected) || rejected.Reasons[0] != verify.ReasonTooLarge {
			t.Errorf("err = %v, want too_large", err)
		}
	})

	t.Run("repeat after acceptance is harmless", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		e.upload(c.Verification.ID, "selfie", faces.GoodFace)
		first, _ := e.svc.CheckPhoto(ctx, c.Verification.ID)
		again, err := e.svc.CheckPhoto(ctx, c.Verification.ID)
		if err != nil || again.Status != verify.StatusPhotoOK || again.Version != first.Version {
			t.Errorf("repeat = %q v%d, %v; want photo_ok unchanged", again.Status, again.Version, err)
		}
	})

	t.Run("after liveness started", func(t *testing.T) {
		e := newEnv(t)
		id, _ := e.toLiveness(t)
		if _, err := e.svc.CheckPhoto(ctx, id); !errors.Is(err, verify.ErrInvalidTransition) {
			t.Errorf("CheckPhoto after liveness started: err = %v, want ErrInvalidTransition", err)
		}
	})
}

func TestStartLiveness(t *testing.T) {
	t.Run("before the photo is checked", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		if _, _, err := e.svc.StartLiveness(ctx, c.Verification.ID); !errors.Is(err, verify.ErrInvalidTransition) {
			t.Errorf("err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("a retry gets a new session, up to the limit", func(t *testing.T) {
		e := newEnv(t)
		id, first := e.toLiveness(t)

		second, v, err := e.svc.StartLiveness(ctx, id)
		if err != nil || second == first || v.LivenessSessionID != second {
			t.Fatalf("retry = %q (first %q), stored %q, %v", second, first, v.LivenessSessionID, err)
		}
		if _, _, err := e.svc.StartLiveness(ctx, id); err != nil {
			t.Fatalf("third attempt: %v", err)
		}
		if _, _, err := e.svc.StartLiveness(ctx, id); !errors.Is(err, verify.ErrTooManyAttempts) {
			t.Errorf("fourth attempt err = %v, want ErrTooManyAttempts", err)
		}
		if n := e.faces.Sessions(); n != 3 {
			t.Errorf("%d sessions created, want 3", n)
		}
	})

	t.Run("only the latest session counts", func(t *testing.T) {
		e := newEnv(t)
		id, old := e.toLiveness(t)
		latest, _, _ := e.svc.StartLiveness(ctx, id)

		// The old session finishing well must not decide anything.
		e.faces.FinishSession(old, verify.LivenessSucceeded, 99)
		if _, err := e.svc.Complete(ctx, id); !errors.Is(err, verify.ErrLivenessNotFinished) {
			t.Errorf("err = %v, want ErrLivenessNotFinished (latest session not finished)", err)
		}
		e.faces.FinishSession(latest, verify.LivenessSucceeded, 97)
		if v, err := e.svc.Complete(ctx, id); err != nil || v.Status != verify.StatusApproved {
			t.Errorf("Complete = %q, %v", v.Status, err)
		}
	})
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name        string
		status      verify.LivenessStatus
		liveness    float64
		match       verify.Match
		wantStatus  verify.Status
		wantReasons []verify.Reason
	}{
		{"someone else's photo", verify.LivenessSucceeded, 98.1, verify.Match{FaceFound: true, Similarity: 12.7},
			verify.StatusRejected, []verify.Reason{verify.ReasonFaceMismatch}},
		{"photo held up to the camera", verify.LivenessSucceeded, 4.3, verify.Match{FaceFound: true, Similarity: 97},
			verify.StatusRejected, []verify.Reason{verify.ReasonLivenessFailed}},
		{"close but not clear", verify.LivenessSucceeded, 93, verify.Match{FaceFound: true, Similarity: 86.4},
			verify.StatusReview, []verify.Reason{verify.ReasonMatchUncertain}},
		{"no face in the reference image", verify.LivenessSucceeded, 95, verify.Match{FaceFound: false},
			verify.StatusRejected, []verify.Reason{verify.ReasonNoFaceInReference}},
		{"session failed", verify.LivenessFailed, 0, verify.Match{},
			verify.StatusRejected, []verify.Reason{verify.ReasonLivenessSessionFailed}},
		{"session expired", verify.LivenessExpired, 0, verify.Match{},
			verify.StatusRejected, []verify.Reason{verify.ReasonLivenessSessionFailed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			id, session := e.toLiveness(t)
			e.faces.FinishSession(session, tt.status, tt.liveness)
			e.faces.SetMatch(tt.match)

			v, err := e.svc.Complete(ctx, id)
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if v.Status != tt.wantStatus || !slices.Equal(v.Reasons, tt.wantReasons) {
				t.Errorf("got %q %v, want %q %v", v.Status, v.Reasons, tt.wantStatus, tt.wantReasons)
			}
		})
	}

	t.Run("not finished yet changes nothing", func(t *testing.T) {
		e := newEnv(t)
		id, _ := e.toLiveness(t)
		if _, err := e.svc.Complete(ctx, id); !errors.Is(err, verify.ErrLivenessNotFinished) {
			t.Fatalf("err = %v, want ErrLivenessNotFinished", err)
		}
		if v, _ := e.store.Get(ctx, id); v.Status != verify.StatusLivenessStarted {
			t.Errorf("status = %q", v.Status)
		}
	})

	t.Run("a second call returns the stored decision", func(t *testing.T) {
		e := newEnv(t)
		id, session := e.toLiveness(t)
		e.faces.FinishSession(session, verify.LivenessSucceeded, 99)
		first, _ := e.svc.Complete(ctx, id)

		// Even if the session's result changed afterwards, the decision stands.
		e.faces.FinishSession(session, verify.LivenessSucceeded, 1)
		second, err := e.svc.Complete(ctx, id)
		if err != nil || second.Status != first.Status || *second.Scores != *first.Scores {
			t.Errorf("second = %q %+v, first = %q %+v (%v)", second.Status, second.Scores, first.Status, first.Scores, err)
		}
	})

	t.Run("photo replaced after the check", func(t *testing.T) {
		e := newEnv(t)
		id, session := e.toLiveness(t)
		e.media.Put(verify.PhotoKey(id), []byte("someone else")) // the upload URL is still valid
		e.faces.FinishSession(session, verify.LivenessSucceeded, 99)

		v, err := e.svc.Complete(ctx, id)
		if err != nil || v.Status != verify.StatusRejected || v.Reasons[0] != verify.ReasonPhotoChanged {
			t.Errorf("got %q %v, %v; want rejected photo_changed", v.Status, v.Reasons, err)
		}
	})

	t.Run("reference image outside the verification's prefix", func(t *testing.T) {
		e := newEnv(t)
		id, session := e.toLiveness(t)
		e.faces.SetResult(session, verify.LivenessResult{
			Status: verify.LivenessSucceeded, Confidence: 99,
			ReferenceKey: "liveness/someone-else/" + session + "/reference.jpg",
		})
		v, err := e.svc.Complete(ctx, id)
		if err != nil || v.Status != verify.StatusRejected || v.ReferenceURL != "" {
			t.Errorf("got %q %v ref %q, %v", v.Status, v.Reasons, v.ReferenceURL, err)
		}
	})

	t.Run("before liveness", func(t *testing.T) {
		e := newEnv(t)
		c, _ := e.svc.Create(ctx)
		if _, err := e.svc.Complete(ctx, c.Verification.ID); !errors.Is(err, verify.ErrInvalidTransition) {
			t.Errorf("err = %v, want ErrInvalidTransition", err)
		}
	})
}

func TestExpiry(t *testing.T) {
	e := newEnv(t)
	c, _ := e.svc.Create(ctx)
	e.now = e.now.Add(7 * 24 * time.Hour) // exactly at ExpiresAt

	if _, err := e.svc.Get(ctx, c.Verification.ID); !errors.Is(err, verify.ErrExpired) {
		t.Errorf("Get err = %v, want ErrExpired", err)
	}
	if _, err := e.svc.CheckPhoto(ctx, c.Verification.ID); !errors.Is(err, verify.ErrExpired) {
		t.Errorf("CheckPhoto err = %v, want ErrExpired", err)
	}
}

func TestNotFound(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.Get(ctx, verify.NewID()); !errors.Is(err, verify.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNewServiceValidates(t *testing.T) {
	if _, err := verify.NewService(verify.Deps{}); err == nil {
		t.Error("NewService with no deps: want error")
	}
	if _, err := verify.NewService(verify.Deps{
		Store: store.NewMemory(), Media: media.NewMemory(), Faces: faces.NewFake(),
	}); err == nil {
		t.Error("NewService with zero Retention: want error")
	}
}

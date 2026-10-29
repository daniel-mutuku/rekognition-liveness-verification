package verify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// The web app re-encodes every photo as JPEG.
	PhotoContentType = "image/jpeg"
	UploadURLTTL     = 5 * time.Minute
	ImageURLTTL      = 10 * time.Minute

	DefaultMaxLivenessAttempts = 3
)

var (
	ErrExpired             = errors.New("verification has expired")
	ErrPhotoMissing        = errors.New("photo has not been uploaded")
	ErrTooManyAttempts     = errors.New("no liveness attempts left")
	ErrLivenessNotFinished = errors.New("liveness session has not finished")
)

// PhotoRejectedError leaves the verification in StatusCreated.
type PhotoRejectedError struct {
	Reasons []Reason
}

func (e *PhotoRejectedError) Error() string {
	return fmt.Sprintf("photo rejected: %v", e.Reasons)
}

// Deps: zero optional fields get the defaults.
type Deps struct {
	Store     Store
	Media     Media
	Faces     Faces
	Retention time.Duration

	Now                 func() time.Time
	NewID               func() string
	Thresholds          Thresholds
	PhotoRules          PhotoRules
	MaxLivenessAttempts int
}

// Service makes every decision from data it fetched itself; a client only
// ever supplies the verification ID.
type Service struct {
	store       Store
	media       Media
	faces       Faces
	retention   time.Duration
	now         func() time.Time
	newID       func() string
	thresholds  Thresholds
	photoRules  PhotoRules
	maxAttempts int
}

func NewService(d Deps) (*Service, error) {
	if d.Store == nil || d.Media == nil || d.Faces == nil {
		return nil, errors.New("verify: Store, Media and Faces are required")
	}
	if d.Retention <= 0 {
		return nil, errors.New("verify: Retention must be positive")
	}
	s := &Service{
		store: d.Store, media: d.Media, faces: d.Faces, retention: d.Retention,
		now: d.Now, newID: d.NewID, thresholds: d.Thresholds, photoRules: d.PhotoRules,
		maxAttempts: d.MaxLivenessAttempts,
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.newID == nil {
		s.newID = NewID
	}
	if s.thresholds == (Thresholds{}) {
		s.thresholds = DefaultThresholds
	}
	if s.photoRules == (PhotoRules{}) {
		s.photoRules = DefaultPhotoRules
	}
	if s.maxAttempts == 0 {
		s.maxAttempts = DefaultMaxLivenessAttempts
	}
	return s, nil
}

func (s *Service) MaxLivenessAttempts() int { return s.maxAttempts }

type Created struct {
	Verification    Verification
	UploadURL       string
	UploadHeaders   map[string]string
	UploadExpiresAt time.Time
}

type View struct {
	Verification
	PhotoURL            string
	ReferenceURL        string
	Thresholds          Thresholds
	MaxLivenessAttempts int
}

func (s *Service) Create(ctx context.Context) (Created, error) {
	now := s.now().UTC()
	v := Verification{
		ID:        s.newID(),
		Status:    StatusCreated,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(s.retention).Truncate(time.Second),
	}
	// Presigning is local; doing it first avoids an orphan record if it fails.
	url, err := s.media.PresignPut(ctx, PhotoKey(v.ID), PhotoContentType, UploadURLTTL)
	if err != nil {
		return Created{}, fmt.Errorf("presign upload: %w", err)
	}
	if err := s.store.Create(ctx, &v); err != nil {
		return Created{}, fmt.Errorf("create verification: %w", err)
	}
	return Created{
		Verification:    v,
		UploadURL:       url,
		UploadHeaders:   map[string]string{"Content-Type": PhotoContentType},
		UploadExpiresAt: now.Add(UploadURLTTL),
	}, nil
}

func (s *Service) Get(ctx context.Context, id string) (View, error) {
	v, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, v)
}

func (s *Service) CheckPhoto(ctx context.Context, id string) (Verification, error) {
	v, err := s.load(ctx, id)
	if err != nil {
		return Verification{}, err
	}
	if v.Status == StatusPhotoOK {
		return v, nil
	}
	if v.Status != StatusCreated {
		return Verification{}, fmt.Errorf("%w: photo check in status %s", ErrInvalidTransition, v.Status)
	}

	key := PhotoKey(id)
	info, err := s.media.Stat(ctx, key)
	if errors.Is(err, ErrObjectMissing) {
		return Verification{}, ErrPhotoMissing
	}
	if err != nil {
		return Verification{}, fmt.Errorf("stat photo: %w", err)
	}
	if info.Size > s.photoRules.MaxBytes {
		return Verification{}, &PhotoRejectedError{Reasons: []Reason{ReasonTooLarge}}
	}

	faces, err := s.faces.DetectFaces(ctx, key)
	if errors.Is(err, ErrUnreadableImage) {
		return Verification{}, &PhotoRejectedError{Reasons: []Reason{ReasonUnreadable}}
	}
	if err != nil {
		return Verification{}, fmt.Errorf("detect faces: %w", err)
	}
	if reasons := s.photoRules.Check(faces); len(reasons) > 0 {
		return Verification{}, &PhotoRejectedError{Reasons: reasons}
	}

	v.PhotoETag = info.ETag
	if err := v.moveTo(StatusPhotoOK, s.now().UTC()); err != nil {
		return Verification{}, err
	}
	if err := s.store.Update(ctx, &v); err != nil {
		return Verification{}, err
	}
	return v, nil
}

// StartLiveness always creates a fresh session; each call uses an attempt.
func (s *Service) StartLiveness(ctx context.Context, id string) (string, Verification, error) {
	v, err := s.load(ctx, id)
	if err != nil {
		return "", Verification{}, err
	}
	if v.Status != StatusPhotoOK && v.Status != StatusLivenessStarted {
		return "", Verification{}, fmt.Errorf("%w: liveness in status %s", ErrInvalidTransition, v.Status)
	}
	if v.LivenessAttempts >= s.maxAttempts {
		return "", Verification{}, ErrTooManyAttempts
	}

	sessionID, err := s.faces.CreateLivenessSession(ctx, LivenessPrefix(id))
	if err != nil {
		return "", Verification{}, fmt.Errorf("create liveness session: %w", err)
	}
	v.LivenessSessionID = sessionID
	v.LivenessAttempts++
	if err := v.moveTo(StatusLivenessStarted, s.now().UTC()); err != nil {
		return "", Verification{}, err
	}
	// On a race the loser's session is never used; it expires at no cost.
	if err := s.store.Update(ctx, &v); err != nil {
		return "", Verification{}, err
	}
	return sessionID, v, nil
}

// Complete uses the session stored on the record, never one from the client.
// Once decided, repeated calls return the stored decision.
func (s *Service) Complete(ctx context.Context, id string) (View, error) {
	v, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	if v.Status.Final() {
		return s.view(ctx, v)
	}
	if v.Status != StatusLivenessStarted {
		return View{}, fmt.Errorf("%w: complete in status %s", ErrInvalidTransition, v.Status)
	}

	d, err := s.evaluate(ctx, &v)
	if err != nil {
		return View{}, err
	}
	v.Reasons = d.Reasons
	if err := v.moveTo(d.Status, s.now().UTC()); err != nil {
		return View{}, err
	}
	if err := s.store.Update(ctx, &v); err != nil {
		// A double submit lost the race; return the winner's decision.
		if errors.Is(err, ErrConflict) {
			if latest, lerr := s.load(ctx, id); lerr == nil && latest.Status.Final() {
				return s.view(ctx, latest)
			}
		}
		return View{}, err
	}
	return s.view(ctx, v)
}

// evaluate fills in v.Scores and v.ReferenceKey as it goes.
func (s *Service) evaluate(ctx context.Context, v *Verification) (Decision, error) {
	res, err := s.faces.LivenessResult(ctx, v.LivenessSessionID)
	if err != nil {
		return Decision{}, fmt.Errorf("liveness result: %w", err)
	}
	switch res.Status {
	case LivenessCreated, LivenessInProgress:
		return Decision{}, ErrLivenessNotFinished
	case LivenessSucceeded:
	default: // FAILED, EXPIRED, or anything new
		return reject(ReasonLivenessSessionFailed), nil
	}
	v.Scores = &Scores{Liveness: res.Confidence}

	if res.ReferenceKey == "" || !strings.HasPrefix(res.ReferenceKey, LivenessPrefix(v.ID)) {
		return reject(ReasonNoFaceInReference), nil
	}
	v.ReferenceKey = res.ReferenceKey

	photo := PhotoKey(v.ID)
	info, err := s.media.Stat(ctx, photo)
	if err != nil && !errors.Is(err, ErrObjectMissing) {
		return Decision{}, fmt.Errorf("stat photo: %w", err)
	}
	if err != nil || info.ETag != v.PhotoETag {
		return reject(ReasonPhotoChanged), nil
	}

	m, err := s.faces.CompareFaces(ctx, photo, res.ReferenceKey)
	if err != nil {
		return Decision{}, fmt.Errorf("compare faces: %w", err)
	}
	if !m.FaceFound {
		return reject(ReasonNoFaceInReference), nil
	}
	v.Scores.Similarity = m.Similarity
	return s.thresholds.Decide(*v.Scores), nil
}

func reject(r Reason) Decision {
	return Decision{Status: StatusRejected, Reasons: []Reason{r}}
}

// load checks expiry itself: DynamoDB TTL can take days to delete an item.
func (s *Service) load(ctx context.Context, id string) (Verification, error) {
	v, err := s.store.Get(ctx, id)
	if err != nil {
		return Verification{}, err
	}
	if v.Expired(s.now()) {
		return Verification{}, ErrExpired
	}
	return v, nil
}

func (s *Service) view(ctx context.Context, v Verification) (View, error) {
	out := View{Verification: v, Thresholds: s.thresholds, MaxLivenessAttempts: s.maxAttempts}
	var err error
	if v.Status != StatusCreated {
		if out.PhotoURL, err = s.media.PresignGet(ctx, PhotoKey(v.ID), ImageURLTTL); err != nil {
			return View{}, fmt.Errorf("presign photo: %w", err)
		}
	}
	if v.ReferenceKey != "" {
		if out.ReferenceURL, err = s.media.PresignGet(ctx, v.ReferenceKey, ImageURLTTL); err != nil {
			return View{}, fmt.Errorf("presign reference: %w", err)
		}
	}
	return out, nil
}

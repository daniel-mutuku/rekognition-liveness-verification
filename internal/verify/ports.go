package verify

import (
	"context"
	"errors"
	"time"
)

// Media is object storage (S3). Keys are always built by this package.
type Media interface {
	// PresignPut signs contentType, so the upload must send exactly that.
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Stat returns ErrObjectMissing if the key doesn't exist.
	Stat(ctx context.Context, key string) (ObjectInfo, error)
}

type ObjectInfo struct {
	Size int64
	ETag string
}

// Faces is face analysis (Rekognition).
type Faces interface {
	// DetectFaces returns ErrUnreadableImage if the object isn't a JPEG or PNG.
	DetectFaces(ctx context.Context, key string) ([]Face, error)
	CreateLivenessSession(ctx context.Context, outputPrefix string) (string, error)
	LivenessResult(ctx context.Context, sessionID string) (LivenessResult, error)
	// CompareFaces compares the largest face in source with the faces in target.
	CompareFaces(ctx context.Context, sourceKey, targetKey string) (Match, error)
}

type Face struct {
	Confidence float64
	Yaw        float64 // degrees; 0 faces the camera
	Pitch      float64
	Brightness float64
	Sharpness  float64
}

type LivenessStatus string

const (
	LivenessCreated    LivenessStatus = "CREATED"
	LivenessInProgress LivenessStatus = "IN_PROGRESS"
	LivenessSucceeded  LivenessStatus = "SUCCEEDED"
	LivenessFailed     LivenessStatus = "FAILED"
	LivenessExpired    LivenessStatus = "EXPIRED"
)

type LivenessResult struct {
	Status       LivenessStatus
	Confidence   float64
	ReferenceKey string // empty if Rekognition wrote no reference image
}

type Match struct {
	FaceFound  bool // false if the target image has no face
	Similarity float64
}

var (
	ErrObjectMissing   = errors.New("object does not exist")
	ErrUnreadableImage = errors.New("image is not a readable JPEG or PNG")
)

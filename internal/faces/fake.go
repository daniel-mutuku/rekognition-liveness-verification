// Package faces implements verify.Faces with Rekognition and a scriptable fake.
package faces

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

// GoodFace is a face that passes verify.DefaultPhotoRules.
var GoodFace = verify.Face{Confidence: 99.9, Yaw: 3, Pitch: -4, Brightness: 72, Sharpness: 55}

type Fake struct {
	mu       sync.Mutex
	photos   map[string][]verify.Face // missing key = unreadable image
	prefixes map[string]string
	results  map[string]verify.LivenessResult
	match    verify.Match
	next     int
}

var _ verify.Faces = (*Fake)(nil)

func NewFake() *Fake {
	return &Fake{
		photos:   make(map[string][]verify.Face),
		prefixes: make(map[string]string),
		results:  make(map[string]verify.LivenessResult),
		match:    verify.Match{FaceFound: true, Similarity: 99.5},
	}
}

func (f *Fake) SetPhotoFaces(key string, faces ...verify.Face) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.photos[key] = faces
}

func (f *Fake) SetMatch(m verify.Match) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.match = m
}

// FinishSession stands in for the browser's stream; a successful session gets
// a reference key where Rekognition would write it.
func (f *Fake) FinishSession(sessionID string, status verify.LivenessStatus, confidence float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := verify.LivenessResult{Status: status, Confidence: confidence}
	if status == verify.LivenessSucceeded {
		r.ReferenceKey = f.prefixes[sessionID] + sessionID + "/reference.jpg"
	}
	f.results[sessionID] = r
}

func (f *Fake) SetResult(sessionID string, r verify.LivenessResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[sessionID] = r
}

func (f *Fake) Sessions() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.next
}

func (f *Fake) DetectFaces(_ context.Context, key string) ([]verify.Face, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	faces, ok := f.photos[key]
	if !ok {
		return nil, verify.ErrUnreadableImage
	}
	return faces, nil
}

func (f *Fake) CreateLivenessSession(_ context.Context, outputPrefix string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := fmt.Sprintf("fake-session-%d", f.next)
	f.prefixes[id] = outputPrefix
	return id, nil
}

func (f *Fake) LivenessResult(_ context.Context, sessionID string) (verify.LivenessResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.results[sessionID]; ok {
		return r, nil
	}
	if _, ok := f.prefixes[sessionID]; ok {
		return verify.LivenessResult{Status: verify.LivenessCreated}, nil
	}
	return verify.LivenessResult{}, errors.New("fake: session not found")
}

func (f *Fake) CompareFaces(context.Context, string, string) (verify.Match, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.match, nil
}

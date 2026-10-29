// Package media implements verify.Media with S3 and an in-memory fake.
package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

// Memory's URLs use the reserved .invalid TLD; tests call Put instead of uploading.
type Memory struct {
	mu      sync.Mutex
	objects map[string][]byte
}

var _ verify.Media = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{objects: make(map[string][]byte)}
}

func (m *Memory) Put(key string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = append([]byte(nil), data...)
}

func (m *Memory) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (string, error) {
	q := url.Values{"method": {"PUT"}, "content-type": {contentType}, "ttl": {ttl.String()}}
	return "https://media.invalid/" + key + "?" + q.Encode(), nil
}

func (m *Memory) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	return "https://media.invalid/" + key + "?" + url.Values{"ttl": {ttl.String()}}.Encode(), nil
}

func (m *Memory) Stat(_ context.Context, key string) (verify.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return verify.ObjectInfo{}, verify.ErrObjectMissing
	}
	return verify.ObjectInfo{Size: int64(len(data)), ETag: fmt.Sprintf("%x", sha256.Sum256(data))[:32]}, nil
}

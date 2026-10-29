package verify

import (
	"crypto/rand"
	"fmt"
)

func PhotoKey(id string) string { return "profiles/" + id + "/photo.jpg" }

// LivenessPrefix is the Face Liveness output prefix; Rekognition adds
// {sessionId}/reference.jpg and audit images under it.
func LivenessPrefix(id string) string { return "liveness/" + id + "/" }

// NewID returns a random v4 UUID. The ID is the only access token for a
// verification, so it has to be unguessable.
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 9562 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ValidID accepts only the lowercase form NewID produces.
func ValidID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
				return false
			}
		}
	}
	return true
}

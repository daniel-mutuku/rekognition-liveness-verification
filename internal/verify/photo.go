package verify

import "math"

const (
	ReasonNoFace        Reason = "no_face"
	ReasonMultipleFaces Reason = "multiple_faces"
	ReasonFaceTurned    Reason = "face_turned"
	ReasonBlurry        Reason = "blurry"
	ReasonTooDark       Reason = "too_dark"
	ReasonTooLarge      Reason = "too_large" // Rekognition's S3 image limit is 15 MB
	ReasonUnreadable    Reason = "unreadable"
)

type PhotoRules struct {
	// Detections below this don't count as faces (posters, passers-by).
	CountConfidence float64
	MinConfidence   float64
	MaxYaw          float64
	MaxPitch        float64
	MinSharpness    float64
	MinBrightness   float64
	MaxBytes        int64
}

var DefaultPhotoRules = PhotoRules{
	CountConfidence: 90,
	MinConfidence:   99,
	MaxYaw:          30,
	MaxPitch:        30,
	MinSharpness:    10,
	MinBrightness:   20,
	MaxBytes:        15 << 20,
}

// Check returns every problem with the face at once, so one new photo can fix them all.
func (r PhotoRules) Check(faces []Face) []Reason {
	var counted []Face
	for _, f := range faces {
		if f.Confidence >= r.CountConfidence {
			counted = append(counted, f)
		}
	}
	switch {
	case len(counted) == 0:
		return []Reason{ReasonNoFace}
	case len(counted) > 1:
		return []Reason{ReasonMultipleFaces}
	}

	f := counted[0]
	if f.Confidence < r.MinConfidence {
		return []Reason{ReasonNoFace}
	}
	var reasons []Reason
	if math.Abs(f.Yaw) > r.MaxYaw || math.Abs(f.Pitch) > r.MaxPitch {
		reasons = append(reasons, ReasonFaceTurned)
	}
	if f.Sharpness < r.MinSharpness {
		reasons = append(reasons, ReasonBlurry)
	}
	if f.Brightness < r.MinBrightness {
		reasons = append(reasons, ReasonTooDark)
	}
	return reasons
}

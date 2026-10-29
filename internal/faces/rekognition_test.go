package faces

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

func TestF64(t *testing.T) {
	for _, tt := range []struct {
		in   *float32
		want float64
	}{
		{nil, 0},
		{aws.Float32(99.2), 99.2},
		{aws.Float32(12.7), 12.7},
		{aws.Float32(100), 100},
	} {
		if got := f64(tt.in); got != tt.want {
			t.Errorf("f64(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestToFace(t *testing.T) {
	got := toFace(types.FaceDetail{
		Confidence: aws.Float32(99.9),
		Pose:       &types.Pose{Yaw: aws.Float32(-12.5), Pitch: aws.Float32(4), Roll: aws.Float32(30)},
		Quality:    &types.ImageQuality{Brightness: aws.Float32(70.5), Sharpness: aws.Float32(53.3)},
	})
	want := verify.Face{Confidence: 99.9, Yaw: -12.5, Pitch: 4, Brightness: 70.5, Sharpness: 53.3}
	if got != want {
		t.Errorf("toFace = %+v, want %+v", got, want)
	}
	// Missing pose/quality must not panic on a nil pointer.
	if got := toFace(types.FaceDetail{Confidence: aws.Float32(50)}); got != (verify.Face{Confidence: 50}) {
		t.Errorf("toFace(sparse) = %+v", got)
	}
}

func TestToLivenessResult(t *testing.T) {
	ref := func(bucket, key string) *types.AuditImage {
		return &types.AuditImage{S3Object: &types.S3Object{Bucket: aws.String(bucket), Name: aws.String(key)}}
	}
	tests := []struct {
		name string
		out  rekognition.GetFaceLivenessSessionResultsOutput
		want verify.LivenessResult
	}{
		{"succeeded", rekognition.GetFaceLivenessSessionResultsOutput{
			Status: types.LivenessSessionStatusSucceeded, Confidence: aws.Float32(98.4),
			ReferenceImage: ref("media", "liveness/v1/s1/reference.jpg"),
		}, verify.LivenessResult{Status: verify.LivenessSucceeded, Confidence: 98.4, ReferenceKey: "liveness/v1/s1/reference.jpg"}},
		{"reference in another bucket is ignored", rekognition.GetFaceLivenessSessionResultsOutput{
			Status: types.LivenessSessionStatusSucceeded, Confidence: aws.Float32(98.4),
			ReferenceImage: ref("someone-elses", "liveness/v1/s1/reference.jpg"),
		}, verify.LivenessResult{Status: verify.LivenessSucceeded, Confidence: 98.4}},
		{"expired, no confidence", rekognition.GetFaceLivenessSessionResultsOutput{
			Status: types.LivenessSessionStatusExpired,
		}, verify.LivenessResult{Status: verify.LivenessExpired}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toLivenessResult(&tt.out, "media"); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToMatch(t *testing.T) {
	match := func(s float32) types.CompareFacesMatch { return types.CompareFacesMatch{Similarity: aws.Float32(s)} }

	if got := toMatch(&rekognition.CompareFacesOutput{
		FaceMatches: []types.CompareFacesMatch{match(97.8), match(10)},
	}); got != (verify.Match{FaceFound: true, Similarity: 97.8}) {
		t.Errorf("best match: %+v", got)
	}
	if got := toMatch(&rekognition.CompareFacesOutput{}); got.FaceFound {
		t.Errorf("no faces: %+v", got)
	}
}

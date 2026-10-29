package faces

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

const auditImagesLimit = 2

type Rekognition struct {
	client *rekognition.Client
	bucket string
}

var _ verify.Faces = (*Rekognition)(nil)

func NewRekognition(cfg aws.Config, bucket string) *Rekognition {
	return &Rekognition{client: rekognition.NewFromConfig(cfg), bucket: bucket}
}

func (r *Rekognition) image(key string) *types.Image {
	return &types.Image{S3Object: &types.S3Object{Bucket: &r.bucket, Name: &key}}
}

func (r *Rekognition) DetectFaces(ctx context.Context, key string) ([]verify.Face, error) {
	out, err := r.client.DetectFaces(ctx, &rekognition.DetectFacesInput{
		Image:      r.image(key),
		Attributes: []types.Attribute{types.AttributeDefault},
	})
	var badFormat *types.InvalidImageFormatException
	if errors.As(err, &badFormat) {
		return nil, verify.ErrUnreadableImage
	}
	if err != nil {
		return nil, fmt.Errorf("rekognition DetectFaces: %w", err)
	}
	faces := make([]verify.Face, 0, len(out.FaceDetails))
	for _, d := range out.FaceDetails {
		faces = append(faces, toFace(d))
	}
	return faces, nil
}

func (r *Rekognition) CreateLivenessSession(ctx context.Context, outputPrefix string) (string, error) {
	out, err := r.client.CreateFaceLivenessSession(ctx, &rekognition.CreateFaceLivenessSessionInput{
		Settings: &types.CreateFaceLivenessSessionRequestSettings{
			OutputConfig: &types.LivenessOutputConfig{
				S3Bucket:    &r.bucket,
				S3KeyPrefix: &outputPrefix,
			},
			AuditImagesLimit: aws.Int32(auditImagesLimit),
		},
	})
	if err != nil {
		return "", fmt.Errorf("rekognition CreateFaceLivenessSession: %w", err)
	}
	return aws.ToString(out.SessionId), nil
}

func (r *Rekognition) LivenessResult(ctx context.Context, sessionID string) (verify.LivenessResult, error) {
	out, err := r.client.GetFaceLivenessSessionResults(ctx, &rekognition.GetFaceLivenessSessionResultsInput{
		SessionId: &sessionID,
	})
	if err != nil {
		return verify.LivenessResult{}, fmt.Errorf("rekognition GetFaceLivenessSessionResults: %w", err)
	}
	return toLivenessResult(out, r.bucket), nil
}

func (r *Rekognition) CompareFaces(ctx context.Context, sourceKey, targetKey string) (verify.Match, error) {
	out, err := r.client.CompareFaces(ctx, &rekognition.CompareFacesInput{
		SourceImage: r.image(sourceKey),
		TargetImage: r.image(targetKey),
		// The default (80) hides low matches; the service applies its own thresholds.
		SimilarityThreshold: aws.Float32(0),
		QualityFilter:       types.QualityFilterNone,
	})
	if err != nil {
		return verify.Match{}, fmt.Errorf("rekognition CompareFaces: %w", err)
	}
	return toMatch(out), nil
}

func toFace(d types.FaceDetail) verify.Face {
	f := verify.Face{Confidence: f64(d.Confidence)}
	if d.Pose != nil {
		f.Yaw, f.Pitch = f64(d.Pose.Yaw), f64(d.Pose.Pitch)
	}
	if d.Quality != nil {
		f.Brightness, f.Sharpness = f64(d.Quality.Brightness), f64(d.Quality.Sharpness)
	}
	return f
}

func toLivenessResult(out *rekognition.GetFaceLivenessSessionResultsOutput, bucket string) verify.LivenessResult {
	res := verify.LivenessResult{
		Status:     verify.LivenessStatus(out.Status),
		Confidence: f64(out.Confidence),
	}
	if ref := out.ReferenceImage; ref != nil && ref.S3Object != nil && aws.ToString(ref.S3Object.Bucket) == bucket {
		res.ReferenceKey = aws.ToString(ref.S3Object.Name)
	}
	return res
}

func toMatch(out *rekognition.CompareFacesOutput) verify.Match {
	// Sorted by similarity, best first.
	if len(out.FaceMatches) > 0 {
		return verify.Match{FaceFound: true, Similarity: f64(out.FaceMatches[0].Similarity)}
	}
	return verify.Match{FaceFound: len(out.UnmatchedFaces) > 0}
}

// f64 goes through the float32's shortest decimal form, so 99.2 doesn't
// become 99.19999694824219.
func f64(p *float32) float64 {
	if p == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strconv.FormatFloat(float64(*p), 'g', -1, 32), 64)
	return v
}

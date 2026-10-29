package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

var _ verify.Media = (*S3)(nil)

func NewS3(cfg aws.Config, bucket string) *S3 {
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// A default checksum signed into a presigned PUT would break browser uploads.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: bucket}
}

func (m *S3) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := m.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &m.bucket,
		Key:         &key,
		ContentType: &contentType,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put %s: %w", key, err)
	}
	return req.URL, nil
}

func (m *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := m.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &m.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return req.URL, nil
}

func (m *S3) Stat(ctx context.Context, key string) (verify.ObjectInfo, error) {
	out, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &m.bucket, Key: &key})
	// HEAD has no body, so it's NotFound rather than NoSuchKey (and only with s3:ListBucket).
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return verify.ObjectInfo{}, verify.ErrObjectMissing
	}
	if err != nil {
		return verify.ObjectInfo{}, fmt.Errorf("head %s: %w", key, err)
	}
	return verify.ObjectInfo{Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag)}, nil
}

package media

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func TestPresignPut(t *testing.T) {
	cfg := aws.Config{
		Region:      "eu-west-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", "token"),
	}
	m := NewS3(cfg, "face-verify-media-test")

	raw, err := m.PresignPut(context.Background(), "profiles/abc/photo.jpg", "image/jpeg", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()

	if !strings.HasSuffix(u.Path, "/profiles/abc/photo.jpg") {
		t.Errorf("path = %q", u.Path)
	}
	if got := q.Get("X-Amz-Expires"); got != "300" {
		t.Errorf("X-Amz-Expires = %q, want 300", got)
	}
	if h := q.Get("X-Amz-SignedHeaders"); !strings.Contains(h, "content-type") {
		t.Errorf("content-type is not a signed header: %q", h)
	}
	for k := range q {
		if strings.Contains(strings.ToLower(k), "checksum") {
			t.Errorf("presigned URL contains checksum parameter %s", k)
		}
	}
}

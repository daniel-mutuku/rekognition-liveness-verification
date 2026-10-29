package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/faces"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/media"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/store"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type testAPI struct {
	router http.Handler
	svc    *verify.Service
	media  *media.Memory
	faces  *faces.Fake
	now    time.Time
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	a := &testAPI{
		media: media.NewMemory(),
		faces: faces.NewFake(),
		now:   time.Date(2026, 10, 28, 14, 32, 0, 0, time.UTC),
	}
	svc, err := verify.NewService(verify.Deps{
		Store: store.NewMemory(), Media: a.media, Faces: a.faces,
		Retention: 7 * 24 * time.Hour,
		Now:       func() time.Time { return a.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	a.svc = svc
	a.router = NewRouter(discard, svc, nil)
	return a
}

func (a *testAPI) call(t *testing.T, method, path string, wantStatus int, out any) {
	t.Helper()
	rr := httptest.NewRecorder()
	a.router.ServeHTTP(rr, httptest.NewRequest(method, path, nil))
	if rr.Code != wantStatus {
		t.Fatalf("%s %s = %d, want %d\n%s", method, path, rr.Code, wantStatus, rr.Body)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("%s %s Content-Type = %q", method, path, ct)
	}
	if out != nil {
		if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: bad JSON %v\n%s", method, path, err, rr.Body)
		}
	}
}

func (a *testAPI) create(t *testing.T) createdJSON {
	t.Helper()
	var c createdJSON
	a.call(t, "POST", "/api/verifications", http.StatusCreated, &c)
	return c
}

func (a *testAPI) upload(id string, found ...verify.Face) {
	a.media.Put(verify.PhotoKey(id), []byte("jpeg bytes"))
	a.faces.SetPhotoFaces(verify.PhotoKey(id), found...)
}

func TestFullFlow(t *testing.T) {
	a := newTestAPI(t)

	c := a.create(t)
	if c.Status != verify.StatusCreated || c.UploadHeaders["Content-Type"] != "image/jpeg" ||
		!strings.Contains(c.UploadURL, verify.PhotoKey(c.ID)) {
		t.Fatalf("create = %+v", c)
	}
	base := "/api/verifications/" + c.ID

	a.upload(c.ID, faces.GoodFace)
	var photo statusJSON
	a.call(t, "POST", base+"/photo", http.StatusOK, &photo)
	if photo.Status != verify.StatusPhotoOK {
		t.Fatalf("photo = %+v", photo)
	}

	var sess sessionJSON
	a.call(t, "POST", base+"/liveness-session", http.StatusOK, &sess)
	if sess.SessionID == "" || sess.Attempt != 1 || sess.MaxAttempts != 3 {
		t.Fatalf("session = %+v", sess)
	}

	a.faces.FinishSession(sess.SessionID, verify.LivenessSucceeded, 99.2)
	a.faces.SetMatch(verify.Match{FaceFound: true, Similarity: 98.6})

	var done verificationJSON
	a.call(t, "POST", base+"/complete", http.StatusOK, &done)
	if done.Status != verify.StatusApproved || len(done.Reasons) != 0 ||
		*done.LivenessConfidence != 99.2 || *done.Similarity != 98.6 ||
		done.PhotoURL == "" || done.ReferenceURL == "" || done.Thresholds.ApproveSimilarity != 95 {
		t.Fatalf("complete = %+v", done)
	}

	var got verificationJSON
	a.call(t, "GET", base, http.StatusOK, &got)
	if got.Status != verify.StatusApproved {
		t.Errorf("GET status = %q", got.Status)
	}
}

func TestJSONShapes(t *testing.T) {
	a := newTestAPI(t)
	c := a.create(t)

	// Before a decision: reasons is [] (not null), and scores are absent.
	rr := httptest.NewRecorder()
	a.router.ServeHTTP(rr, httptest.NewRequest("GET", "/api/verifications/"+c.ID, nil))
	body := rr.Body.String()
	for _, want := range []string{`"reasons":[]`, `"status":"created"`, `"thresholds":{"approveLiveness":90`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s:\n%s", want, body)
		}
	}
	for _, absent := range []string{"livenessConfidence", "similarity", "photoUrl", "referenceUrl"} {
		if strings.Contains(body, absent) {
			t.Errorf("body should not contain %s yet:\n%s", absent, body)
		}
	}
}

func TestErrors(t *testing.T) {
	a := newTestAPI(t)
	c := a.create(t)
	base := "/api/verifications/" + c.ID

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"id not a UUID", "GET", "/api/verifications/abc", http.StatusBadRequest, "invalid_id"},
		{"path tricks in id", "POST", "/api/verifications/..%2F..%2Fx/photo", http.StatusBadRequest, "invalid_id"},
		{"unknown id", "GET", "/api/verifications/" + verify.NewID(), http.StatusNotFound, "not_found"},
		{"photo before upload", "POST", base + "/photo", http.StatusConflict, "photo_missing"},
		{"liveness before photo", "POST", base + "/liveness-session", http.StatusConflict, "wrong_step"},
		{"complete before liveness", "POST", base + "/complete", http.StatusConflict, "wrong_step"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body errorBody
			a.call(t, tt.method, tt.path, tt.wantStatus, &body)
			if body.Code != tt.wantCode || body.Error == "" {
				t.Errorf("body = %+v, want code %q", body, tt.wantCode)
			}
		})
	}
}

func TestPhotoRejected(t *testing.T) {
	a := newTestAPI(t)
	c := a.create(t)
	blurryDark := faces.GoodFace
	blurryDark.Sharpness, blurryDark.Brightness = 3, 5
	a.upload(c.ID, blurryDark)

	var body errorBody
	a.call(t, "POST", "/api/verifications/"+c.ID+"/photo", http.StatusUnprocessableEntity, &body)
	if body.Code != "photo_rejected" ||
		!slices.Equal(body.Reasons, []verify.Reason{verify.ReasonBlurry, verify.ReasonTooDark}) {
		t.Errorf("body = %+v", body)
	}
}

func TestRejectedResultShowsReasons(t *testing.T) {
	a := newTestAPI(t)
	c := a.create(t)
	base := "/api/verifications/" + c.ID
	a.upload(c.ID, faces.GoodFace)
	a.call(t, "POST", base+"/photo", http.StatusOK, nil)
	var sess sessionJSON
	a.call(t, "POST", base+"/liveness-session", http.StatusOK, &sess)
	a.faces.FinishSession(sess.SessionID, verify.LivenessSucceeded, 98.1)
	a.faces.SetMatch(verify.Match{FaceFound: false})

	var done verificationJSON
	a.call(t, "POST", base+"/complete", http.StatusOK, &done)
	if done.Status != verify.StatusRejected ||
		!slices.Equal(done.Reasons, []verify.Reason{verify.ReasonNoFaceInReference}) {
		t.Fatalf("complete = %+v", done)
	}
	// Liveness was measured; similarity wasn't, so it's absent rather than 0.
	if done.LivenessConfidence == nil || done.Similarity != nil {
		t.Errorf("scores = %v %v, want liveness only", done.LivenessConfidence, done.Similarity)
	}
}

func TestExpiredIsGone(t *testing.T) {
	a := newTestAPI(t)
	c := a.create(t)
	a.now = a.now.Add(8 * 24 * time.Hour)

	var body errorBody
	a.call(t, "GET", "/api/verifications/"+c.ID, http.StatusGone, &body)
	if body.Code != "expired" {
		t.Errorf("code = %q", body.Code)
	}
}

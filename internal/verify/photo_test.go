package verify

import (
	"slices"
	"testing"
)

func TestPhotoRulesCheck(t *testing.T) {
	good := Face{Confidence: 99.9, Yaw: 3, Pitch: -4, Brightness: 72, Sharpness: 55}
	with := func(change func(*Face)) Face {
		f := good
		change(&f)
		return f
	}
	background := Face{Confidence: 60} // a poster or a blurry passer-by

	tests := []struct {
		name  string
		faces []Face
		want  []Reason
	}{
		{"one good face", []Face{good}, nil},
		{"good face plus a weak background detection", []Face{good, background}, nil},
		{"no faces", nil, []Reason{ReasonNoFace}},
		{"only a weak detection", []Face{background}, []Reason{ReasonNoFace}},
		{"one face, not clearly a face", []Face{with(func(f *Face) { f.Confidence = 95 })}, []Reason{ReasonNoFace}},
		{"two people", []Face{good, good}, []Reason{ReasonMultipleFaces}},
		{"turned left", []Face{with(func(f *Face) { f.Yaw = -45 })}, []Reason{ReasonFaceTurned}},
		{"looking up", []Face{with(func(f *Face) { f.Pitch = 31 })}, []Reason{ReasonFaceTurned}},
		{"yaw exactly at the limit", []Face{with(func(f *Face) { f.Yaw = 30 })}, nil},
		{"blurry", []Face{with(func(f *Face) { f.Sharpness = 4 })}, []Reason{ReasonBlurry}},
		{"too dark", []Face{with(func(f *Face) { f.Brightness = 8 })}, []Reason{ReasonTooDark}},
		{"several problems at once", []Face{with(func(f *Face) { f.Yaw = 50; f.Sharpness = 2; f.Brightness = 5 })},
			[]Reason{ReasonFaceTurned, ReasonBlurry, ReasonTooDark}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultPhotoRules.Check(tt.faces); !slices.Equal(got, tt.want) {
				t.Errorf("Check = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIDs(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		id := NewID()
		if !ValidID(id) {
			t.Fatalf("NewID() = %q is not a ValidID", id)
		}
		if id[14] != '4' {
			t.Fatalf("NewID() = %q is not version 4", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}

	for _, bad := range []string{
		"",
		"not-a-uuid",
		"0B6F1C2E-8D4A-4F3B-9C1E-2A7D5E6F8A90", // uppercase
		"0b6f1c2e8d4a4f3b9c1e2a7d5e6f8a90",     // no hyphens
		"0b6f1c2e-8d4a-4f3b-9c1e-2a7d5e6f8a9g", // not hex
		"../../profiles/x/photo.jpg-aaaaaaaaa", // 36 chars, path tricks
	} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true", bad)
		}
	}
}

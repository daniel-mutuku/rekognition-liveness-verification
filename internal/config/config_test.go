package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnv(t *testing.T) {
	required := map[string]string{
		"MEDIA_BUCKET":        "media",
		"VERIFICATIONS_TABLE": "verifications",
	}
	with := func(overrides map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range required {
			m[k] = v
		}
		for k, v := range overrides {
			m[k] = v
		}
		return m
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string // substrings that must all appear in the error
	}{
		{
			name: "defaults",
			env:  required,
			want: Config{
				Addr: ":8080", AWSRegion: "eu-west-1",
				MediaBucket: "media", VerificationsTable: "verifications",
				RetentionDays: 7, LogLevel: slog.LevelInfo, WebDir: "web/dist",
			},
		},
		{
			name: "all set",
			env: with(map[string]string{
				"ADDR": "127.0.0.1:9000", "AWS_REGION": "us-east-1",
				"RETENTION_DAYS": "30", "LOG_LEVEL": "DEBUG", "WEB_DIR": "/srv/web",
			}),
			want: Config{
				Addr: "127.0.0.1:9000", AWSRegion: "us-east-1",
				MediaBucket: "media", VerificationsTable: "verifications",
				RetentionDays: 30, LogLevel: slog.LevelDebug, WebDir: "/srv/web",
			},
		},
		{
			name: "blank values fall back to defaults",
			env:  with(map[string]string{"ADDR": "  ", "RETENTION_DAYS": ""}),
			want: Config{
				Addr: ":8080", AWSRegion: "eu-west-1",
				MediaBucket: "media", VerificationsTable: "verifications",
				RetentionDays: 7, LogLevel: slog.LevelInfo, WebDir: "web/dist",
			},
		},
		{
			name:    "missing required",
			env:     map[string]string{},
			wantErr: []string{"MEDIA_BUCKET is required", "VERIFICATIONS_TABLE is required"},
		},
		{
			name:    "retention not a number",
			env:     with(map[string]string{"RETENTION_DAYS": "seven"}),
			wantErr: []string{"RETENTION_DAYS must be a whole number"},
		},
		{
			name:    "retention out of range",
			env:     with(map[string]string{"RETENTION_DAYS": "0"}),
			wantErr: []string{"between 1 and 365, got 0"},
		},
		{
			name:    "unsupported region",
			env:     with(map[string]string{"AWS_REGION": "eu-central-1"}),
			wantErr: []string{`AWS_REGION "eu-central-1" does not support Face Liveness`},
		},
		{
			name:    "bad log level",
			env:     with(map[string]string{"LOG_LEVEL": "loud"}),
			wantErr: []string{"LOG_LEVEL"},
		},
		{
			name: "reports every problem",
			env: map[string]string{
				"AWS_REGION": "mars-1", "RETENTION_DAYS": "999", "LOG_LEVEL": "loud",
			},
			wantErr: []string{
				"MEDIA_BUCKET", "VERIFICATIONS_TABLE", "AWS_REGION", "RETENTION_DAYS", "LOG_LEVEL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromEnv(env(tt.env))

			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected error, got config %+v", got)
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error missing %q:\n%v", want, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestLoadShellOverridesDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "MEDIA_BUCKET=from-file\nVERIFICATIONS_TABLE=table-from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MEDIA_BUCKET", "from-shell")
	// loadDotEnv sets this with os.Setenv, which t.Setenv won't restore.
	if _, set := os.LookupEnv("VERIFICATIONS_TABLE"); set {
		t.Skip("VERIFICATIONS_TABLE is set in the test environment")
	}
	t.Cleanup(func() { os.Unsetenv("VERIFICATIONS_TABLE") })

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MediaBucket != "from-shell" {
		t.Errorf("MediaBucket = %q, want shell value %q", cfg.MediaBucket, "from-shell")
	}
	if cfg.VerificationsTable != "table-from-file" {
		t.Errorf("VerificationsTable = %q, want file value %q", cfg.VerificationsTable, "table-from-file")
	}
}

func TestLoadMissingDotEnvIsFine(t *testing.T) {
	t.Setenv("MEDIA_BUCKET", "m")
	t.Setenv("VERIFICATIONS_TABLE", "v")
	if _, err := Load(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatalf("missing .env should not be an error: %v", err)
	}
}

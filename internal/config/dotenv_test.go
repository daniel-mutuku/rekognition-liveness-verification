package config

import (
	"maps"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr string
	}{
		{
			name: "terraform env_file output",
			input: `# .env (Go server)
AWS_PROFILE=face-verify-server
AWS_REGION=eu-west-1
RETENTION_DAYS=7
`,
			want: map[string]string{
				"AWS_PROFILE": "face-verify-server", "AWS_REGION": "eu-west-1", "RETENTION_DAYS": "7",
			},
		},
		{
			name:  "quotes, export, spacing",
			input: "export A=1\nB = \"two words\"\nC='single'\nD=\"unmatched'\n",
			want:  map[string]string{"A": "1", "B": "two words", "C": "single", "D": `"unmatched'`},
		},
		{
			name:  "empty value and hash inside value",
			input: "EMPTY=\nURL=http://x/#frag\n",
			want:  map[string]string{"EMPTY": "", "URL": "http://x/#frag"},
		},
		{
			name:    "missing equals",
			input:   "OK=1\nnot a pair\n",
			wantErr: "line 2",
		},
		{
			name:    "empty key",
			input:   "=value\n",
			wantErr: "line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDotEnv(strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}

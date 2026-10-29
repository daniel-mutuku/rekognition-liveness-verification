// Package config loads settings from the environment and an optional .env file.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	Addr               string
	AWSRegion          string
	MediaBucket        string
	VerificationsTable string
	RetentionDays      int
	LogLevel           slog.Level
	WebDir             string
}

const (
	defaultAddr          = ":8080"
	defaultRegion        = "eu-west-1"
	defaultRetentionDays = 7
	defaultLogLevel      = "info"
	defaultWebDir        = "web/dist"
)

// Face Liveness is only available in these regions (same list as infra/variables.tf).
var livenessRegions = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1", "ap-south-1"}

// Load applies dotenvPath to the process environment (the AWS SDK reads
// AWS_PROFILE from there) and builds a Config from it.
func Load(dotenvPath string) (Config, error) {
	if err := loadDotEnv(dotenvPath); err != nil {
		return Config{}, err
	}
	return FromEnv(os.Getenv)
}

// FromEnv reports every invalid setting at once rather than one per restart.
func FromEnv(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	var errs []error
	cfg := Config{
		Addr:               get("ADDR", defaultAddr),
		AWSRegion:          get("AWS_REGION", defaultRegion),
		MediaBucket:        get("MEDIA_BUCKET", ""),
		VerificationsTable: get("VERIFICATIONS_TABLE", ""),
		WebDir:             get("WEB_DIR", defaultWebDir),
	}

	if cfg.MediaBucket == "" {
		errs = append(errs, errors.New("MEDIA_BUCKET is required"))
	}
	if cfg.VerificationsTable == "" {
		errs = append(errs, errors.New("VERIFICATIONS_TABLE is required"))
	}
	if !slices.Contains(livenessRegions, cfg.AWSRegion) {
		errs = append(errs, fmt.Errorf("AWS_REGION %q does not support Face Liveness (use one of %s)",
			cfg.AWSRegion, strings.Join(livenessRegions, ", ")))
	}

	days, err := strconv.Atoi(get("RETENTION_DAYS", strconv.Itoa(defaultRetentionDays)))
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("RETENTION_DAYS must be a whole number: %w", err))
	case days < 1 || days > 365:
		errs = append(errs, fmt.Errorf("RETENTION_DAYS must be between 1 and 365, got %d", days))
	default:
		cfg.RetentionDays = days
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", defaultLogLevel))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}

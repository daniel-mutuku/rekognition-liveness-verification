// Command server runs the verification API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/config"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/faces"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/httpapi"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/media"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/store"
	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

const (
	awsCheckTimeout = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	awsCfg, err := loadAWSConfig(ctx, logger, cfg.AWSRegion)
	if err != nil {
		return err
	}

	svc, err := verify.NewService(verify.Deps{
		Store:     store.NewDynamo(awsCfg, cfg.VerificationsTable),
		Media:     media.NewS3(awsCfg, cfg.MediaBucket),
		Faces:     faces.NewRekognition(awsCfg, cfg.MediaBucket),
		Retention: time.Duration(cfg.RetentionDays) * 24 * time.Hour,
	})
	if err != nil {
		return err
	}

	// Without a build (make dev, where Vite serves the app) the server is API-only.
	var web http.Handler
	if info, err := os.Stat(cfg.WebDir); err == nil && info.IsDir() {
		web = httpapi.NewSPA(os.DirFS(cfg.WebDir))
		logger.Info("serving web app", "dir", cfg.WebDir)
	} else {
		logger.Info("no web app build, serving the API only", "dir", cfg.WebDir, "hint", "make web-build")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(logger, svc, web),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	stop() // a second Ctrl+C kills the process if shutdown hangs
	logger.Info("shutting down", "timeout", shutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

// loadAWSConfig fails fast at startup if the credentials don't work, and logs
// which role the server is running as.
func loadAWSConfig(ctx context.Context, logger *slog.Logger, region string) (aws.Config, error) {
	ctx, cancel := context.WithTimeout(ctx, awsCheckTimeout)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}

	out, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return aws.Config{}, fmt.Errorf("AWS credentials don't work (AWS_PROFILE=%q): %w\n"+
			"hint: check `aws sts get-caller-identity --profile <profile>` works, "+
			"and that ~/.aws/config has the profile from `terraform output -raw aws_config_snippet`",
			os.Getenv("AWS_PROFILE"), err)
	}

	logger.Info("aws identity",
		"arn", aws.ToString(out.Arn),
		"account", aws.ToString(out.Account),
		"region", region,
	)
	return awsCfg, nil
}

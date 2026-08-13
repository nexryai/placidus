// Command placidus starts the Mirakurun-only web television server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nexryai/placidus/internal/config"
	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/recording"
	"github.com/nexryai/placidus/internal/transcode"
	"github.com/nexryai/placidus/internal/tuner"
	"github.com/nexryai/placidus/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("placidus stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	logger.Info("placidus starting",
		"listen_address", cfg.ListenAddress,
		"mirakurun_host", sanitizedHost(cfg.MirakurunURL),
		"data_directory", cfg.DataDirectory,
		"recording_directory", cfg.RecordingDir,
		"ffmpeg_path", cfg.FFmpegPath,
		"preferred_encoder", cfg.Encoder,
		"gpu_device", cfg.GPUDevice,
		"tuner_limit", cfg.TunerLimit,
		"recording_margin", cfg.ScheduleMargin,
		"log_level", cfg.LogLevel,
	)
	if err := os.MkdirAll(cfg.DataDirectory, 0o755); err != nil {
		return err
	}
	logger.Debug("data directory ready", "path", cfg.DataDirectory)

	backend, err := mirakurun.NewClient(cfg.MirakurunURL, nil, logger.With("component", "mirakurun"))
	if err != nil {
		return err
	}
	store, err := recording.NewStore(filepath.Join(cfg.DataDirectory, "reservations.json"), logger.With("component", "reservation_store"))
	if err != nil {
		return err
	}
	coordinator := tuner.NewCoordinator(cfg.TunerLimit, logger.With("component", "tuner"))

	probeContext, cancelProbe := context.WithTimeout(context.Background(), 45*time.Second)
	logger.Info("probing FFmpeg encoders", "binary", cfg.FFmpegPath, "preferred_encoder", cfg.Encoder)
	capabilities := transcode.Probe(probeContext, cfg.FFmpegPath, cfg.Encoder, cfg.GPUDevice, logger.With("component", "ffmpeg_probe"))
	cancelProbe()
	if capabilities.Warning != "" {
		logger.Warn("FFmpeg capability warning", "warning", capabilities.Warning, "failures", capabilities.Failures)
	} else {
		logger.Info("FFmpeg ready", "version", capabilities.Version, "encoder", capabilities.Selected.Name, "hardware", capabilities.Selected.Hardware)
	}
	liveManager, err := transcode.NewLiveManager(
		cfg.FFmpegPath, cfg.MirakurunURL, filepath.Join(cfg.DataDirectory, "hls"), capabilities, coordinator, logger.With("component", "live"),
	)
	if err != nil {
		return err
	}
	defer logger.Info("placidus shutdown complete")
	defer liveManager.Close()

	scheduler, err := recording.NewScheduler(store, backend, coordinator, cfg.RecordingDir, cfg.ScheduleMargin, logger.With("component", "scheduler"))
	if err != nil {
		return err
	}
	webServer, err := web.NewServer(backend, store, scheduler, liveManager, coordinator, cfg.RecordingDir, cfg.ScheduleMargin, cfg.TunerLimit, logger.With("component", "http"))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	schedulerDone := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(schedulerDone)
	}()

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           webServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	errorsFromServer := make(chan error, 1)
	go func() {
		logger.Info("placidus listening", "address", cfg.ListenAddress, "mirakurun_host", sanitizedHost(cfg.MirakurunURL))
		errorsFromServer <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested", "reason", ctx.Err())
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			return err
		}
		select {
		case <-schedulerDone:
			logger.Debug("HTTP server and recording scheduler stopped")
		case <-shutdownContext.Done():
			return shutdownContext.Err()
		}
		return nil
	case err := <-errorsFromServer:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		stop()
		<-schedulerDone
		return err
	}
}

func sanitizedHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid"
	}
	return parsed.Host
}

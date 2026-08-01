// Command placidus starts the Mirakurun-only web television server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("placidus stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDirectory, 0o755); err != nil {
		return err
	}

	backend, err := mirakurun.NewClient(cfg.MirakurunURL, nil)
	if err != nil {
		return err
	}
	store, err := recording.NewStore(filepath.Join(cfg.DataDirectory, "reservations.json"))
	if err != nil {
		return err
	}
	coordinator := tuner.NewCoordinator(cfg.TunerLimit)

	probeContext, cancelProbe := context.WithTimeout(context.Background(), 45*time.Second)
	capabilities := transcode.Probe(probeContext, cfg.FFmpegPath, cfg.Encoder)
	cancelProbe()
	if capabilities.Warning != "" {
		logger.Warn("FFmpeg capability warning", "warning", capabilities.Warning, "failures", capabilities.Failures)
	} else {
		logger.Info("FFmpeg ready", "version", capabilities.Version, "encoder", capabilities.Selected.Name, "hardware", capabilities.Selected.Hardware)
	}
	liveManager, err := transcode.NewLiveManager(
		cfg.FFmpegPath, cfg.MirakurunURL, filepath.Join(cfg.DataDirectory, "hls"), capabilities, coordinator, logger,
	)
	if err != nil {
		return err
	}
	defer liveManager.Close()

	scheduler, err := recording.NewScheduler(store, backend, coordinator, cfg.RecordingDir, cfg.ScheduleMargin, logger)
	if err != nil {
		return err
	}
	webServer, err := web.NewServer(backend, store, scheduler, liveManager, coordinator, cfg.RecordingDir, cfg.ScheduleMargin, cfg.TunerLimit, logger)
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
		logger.Info("placidus listening", "address", cfg.ListenAddress, "mirakurun", cfg.MirakurunURL)
		errorsFromServer <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			return err
		}
		select {
		case <-schedulerDone:
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

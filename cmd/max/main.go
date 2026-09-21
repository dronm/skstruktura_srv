package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dronm/skstruktura/internal/config"
	"github.com/dronm/skstruktura/internal/maxbot"
	weblogger "github.com/dronm/webapp/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("MAX bot failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", config.DefaultConfigPath, "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if err := weblogger.Init(cfg.Debug.LogLevel, cfg.Debug.LogFormat); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	if strings.TrimSpace(cfg.MAX.BotToken) == "" {
		return fmt.Errorf("max.bot_token is required")
	}

	pool, err := pgxpool.New(context.Background(), cfg.Database.Primary)
	if err != nil {
		return fmt.Errorf("open MAX database pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("ping MAX database: %w", err)
	}

	client, err := maxbot.NewClient(cfg.MAX.BotToken)
	if err != nil {
		return err
	}
	store := maxbot.NewStore(pool, cfg.MAX.BotToken)
	processor := maxbot.NewProcessor(store, client)

	pollInterval, err := cfg.MAX.SenderPollIntervalDuration()
	if err != nil {
		return err
	}
	notifyReconnectInterval, err := cfg.MAX.SenderNotifyReconnectIntervalDuration()
	if err != nil {
		return err
	}
	lockTimeout, err := cfg.MAX.SenderLockTimeoutDuration()
	if err != nil {
		return err
	}
	retryBaseDelay, err := cfg.MAX.SenderRetryBaseDelayDuration()
	if err != nil {
		return err
	}
	retryMaxDelay, err := cfg.MAX.SenderRetryMaxDelayDuration()
	if err != nil {
		return err
	}
	longPollingTimeout, err := cfg.MAX.LongPollingTimeoutDuration()
	if err != nil {
		return err
	}

	sender := maxbot.NewSender(store, client, maxbot.SenderConfig{
		DSN:                     cfg.Database.Primary,
		PollInterval:            pollInterval,
		NotifyReconnectInterval: notifyReconnectInterval,
		LockTimeout:             lockTimeout,
		RetryBaseDelay:          retryBaseDelay,
		RetryMaxDelay:           retryMaxDelay,
		MaxAttempts:             cfg.MAX.SenderMaxAttempts,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateErr := make(chan error, 1)
	var server *http.Server
	updateMode := strings.TrimSpace(cfg.MAX.UpdateMode)

	switch updateMode {
	case config.MAXUpdateModeWebhook:
		webhookCtx, webhookCancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := client.ConfigureWebhook(webhookCtx, cfg.MAX.WebhookURL, cfg.MAX.WebhookSecret)
		webhookCancel()
		if err != nil {
			return err
		}
		slog.Info("MAX webhook subscription configured", "url", cfg.MAX.WebhookURL)

		handler := maxbot.NewHandler(processor, cfg.MAX.WebhookSecret)
		mux := http.NewServeMux()
		mux.Handle("/webhook", handler)
		server = &http.Server{
			Addr:         cfg.MAX.HTTPAddr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		go func() {
			slog.Info("MAX bot webhook server started", "addr", cfg.MAX.HTTPAddr)
			updateErr <- server.ListenAndServe()
		}()

	case config.MAXUpdateModeLongPolling:
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		removed, err := client.RemoveWebhookSubscriptions(cleanupCtx)
		cleanupCancel()
		if err != nil {
			return fmt.Errorf("prepare MAX Long Polling: %w", err)
		}
		if removed > 0 {
			slog.Info("MAX webhook subscriptions removed for Long Polling", "count", removed)
		}

		poller := maxbot.NewPoller(client, processor, maxbot.PollerConfig{
			Timeout: longPollingTimeout,
			Limit:   cfg.MAX.LongPollingLimit,
		})
		go func() {
			slog.Info(
				"MAX Long Polling started",
				"timeout", longPollingTimeout,
				"limit", cfg.MAX.LongPollingLimit,
			)
			updateErr <- poller.Run(runCtx)
		}()

	default:
		return fmt.Errorf("unsupported MAX update mode %q", updateMode)
	}

	senderErr := make(chan error, 1)
	go func() {
		slog.Info("MAX outgoing sender started")
		senderErr <- sender.Run(runCtx)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	var runErr error
	select {
	case sig := <-stop:
		slog.Info("MAX bot shutdown requested", "signal", sig.String())
	case err := <-updateErr:
		if server != nil && errors.Is(err, http.ErrServerClosed) {
			runErr = nil
		} else if err != nil && !errors.Is(err, context.Canceled) {
			runErr = fmt.Errorf("MAX %s update transport stopped: %w", updateMode, err)
		} else {
			runErr = fmt.Errorf("MAX %s update transport stopped unexpectedly", updateMode)
		}
	case err := <-senderErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			runErr = fmt.Errorf("MAX sender stopped: %w", err)
		} else {
			runErr = fmt.Errorf("MAX sender stopped unexpectedly")
		}
	}

	cancel()
	if server != nil {
		ctx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(ctx); err != nil && runErr == nil {
			return err
		}
	}
	return runErr
}

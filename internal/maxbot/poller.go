package maxbot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const longPollingRetryDelay = 2 * time.Second

type PollerConfig struct {
	Timeout time.Duration
	Limit   int
}

type Poller struct {
	client    *Client
	processor *Processor
	cfg       PollerConfig
}

func NewPoller(client *Client, processor *Processor, cfg PollerConfig) *Poller {
	return &Poller{
		client:    client,
		processor: processor,
		cfg:       cfg,
	}
}

func (p *Poller) Run(ctx context.Context) error {
	if p == nil || p.client == nil || p.processor == nil {
		return fmt.Errorf("MAX long poller is not initialized")
	}

	var marker *int64
	for ctx.Err() == nil {
		page, err := p.client.GetUpdates(ctx, marker, p.cfg.Timeout, p.cfg.Limit)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("poll MAX updates", "error", err)
			if !sleepContext(ctx, longPollingRetryDelay) {
				return ctx.Err()
			}
			continue
		}

		processed := true
		for _, raw := range page.Updates {
			if err := p.processor.ProcessRaw(ctx, raw); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Error("process MAX long polling update", "error", err)
				processed = false
				break
			}
		}
		if !processed {
			// Do not advance the marker. MAX will return the same batch again,
			// allowing a transient database/application failure to be retried.
			if !sleepContext(ctx, longPollingRetryDelay) {
				return ctx.Err()
			}
			continue
		}

		if page.Marker == nil {
			if len(page.Updates) > 0 {
				slog.Warn("MAX long polling response contains updates without a marker")
			}
			if !sleepContext(ctx, longPollingRetryDelay) {
				return ctx.Err()
			}
			continue
		}
		marker = page.Marker
	}
	return ctx.Err()
}

package keepalive

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Pinger sends periodic HTTP requests to keep cloud instances from sleeping.
type Pinger struct {
	targetURL string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewPinger initializes a keep-alive pinger.
func NewPinger(targetURL string, interval time.Duration, logger *slog.Logger) *Pinger {
	if interval < 1*time.Minute {
		interval = 10 * time.Minute // default to every 10 minutes (Render timeout is 15 min)
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL != "" && !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}
	if targetURL != "" && !strings.HasSuffix(targetURL, "/healthz") && !strings.HasSuffix(targetURL, "/health") {
		targetURL = strings.TrimSuffix(targetURL, "/") + "/healthz"
	}

	return &Pinger{
		targetURL: targetURL,
		interval:  interval,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Start runs the periodic ping loop in background until context is canceled.
func (p *Pinger) Start(ctx context.Context) {
	if p.targetURL == "" {
		return
	}

	p.logger.Info("keepalive anti-sleep worker started",
		slog.String("target_url", p.targetURL),
		slog.Duration("interval", p.interval),
	)

	// Wait 1 minute before sending initial ping
	select {
	case <-ctx.Done():
		return
	case <-time.After(1 * time.Minute):
		p.ping(ctx)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("keepalive anti-sleep worker stopped")
			return
		case <-ticker.C:
			p.ping(ctx)
		}
	}
}

func (p *Pinger) ping(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.targetURL, nil)
	if err != nil {
		p.logger.Warn("failed to create keepalive request", slog.String("error", err.Error()))
		return
	}
	req.Header.Set("User-Agent", "JobTracker-KeepAlive/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Warn("keepalive ping failed", slog.String("target", p.targetURL), slog.String("error", err.Error()))
		return
	}
	defer resp.Body.Close()

	p.logger.Info("keepalive ping completed",
		slog.String("target", p.targetURL),
		slog.Int("status", resp.StatusCode),
	)
}

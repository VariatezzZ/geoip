package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultMaxBytes = 128 << 20
	defaultRetries  = 3
	defaultBackoff  = 500 * time.Millisecond
)

type Client struct {
	HTTPClient *http.Client
	MaxBytes   int64
	Retries    int
	Backoff    time.Duration
}

func New() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
		MaxBytes:   defaultMaxBytes,
		Retries:    defaultRetries,
		Backoff:    defaultBackoff,
	}
}

func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	if c == nil {
		c = New()
	}
	if rawURL == "" {
		return nil, errors.New("download URL is empty")
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	attempts := c.Retries
	if attempts <= 0 {
		attempts = 1
	}
	backoff := c.Backoff
	if backoff <= 0 {
		backoff = defaultBackoff
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			wait := backoff
			for index := 2; index < attempt; index++ {
				wait *= 2
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("download %s: %w", rawURL, ctx.Err())
			case <-timer.C:
			}
		}

		data, retry, err := c.getOnce(ctx, client, rawURL, maxBytes)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}

	return nil, fmt.Errorf("download %s: %w", rawURL, lastErr)
}

func (c *Client) getOnce(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("User-Agent", "geoip-builder/1.0")
	request.Header.Set("Accept", "*/*")

	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		return nil, true, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		retry := response.StatusCode == http.StatusRequestTimeout ||
			response.StatusCode == http.StatusTooManyRequests ||
			response.StatusCode >= 500
		return nil, retry, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}

	limited := io.LimitReader(response.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, true, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, false, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	if len(data) == 0 {
		return nil, true, errors.New("response is empty")
	}
	return data, false, nil
}

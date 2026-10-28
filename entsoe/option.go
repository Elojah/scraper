package entsoe

import (
	"net/http"
	"time"
)

// Option configures a Client.
type Option func(c *config)

// WithBaseURL sends requests to u, with no query, rather than BaseURL: an httptest.Server's, or the platform's next.
func WithBaseURL(u string) Option {
	return func(c *config) {
		c.baseURL = u
	}
}

// WithHTTPClient sends requests with hc, rather than a client timing out after a minute.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) {
		c.client = hc
	}
}

type config struct {
	baseURL string
	client  *http.Client
}

func newConfig(options []Option) config {
	c := config{baseURL: BaseURL, client: &http.Client{Timeout: time.Minute}}
	for _, o := range options {
		o(&c)
	}

	return c
}

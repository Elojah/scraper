package entsoe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/elojah/scraper"
)

const (
	// BaseURL is the Transparency Platform's API.
	BaseURL = "https://web-api.tp.entsoe.eu/api"
	// periodLayout is the layout of periodStart and periodEnd, in UTC.
	periodLayout = "200601021504"
	// maxBody bounds an answer read: a year of quarter-hours is a few MB.
	maxBody = 256 << 20
)

// Client queries the Transparency Platform with a security token. It is safe for concurrent use.
type Client struct {
	config

	token string
}

// New returns a Client sending token, which no error it returns holds.
func New(token string, options ...Option) *Client {
	return &Client{token: token, config: newConfig(options)}
}

// get asks query over [from, to), and decodes the answer.
func (c *Client) get(ctx context.Context, query url.Values, from, to time.Time) (map[PSRType]scraper.Series, error) {
	query.Set("periodStart", from.UTC().Format(periodLayout))
	query.Set("periodEnd", to.UTC().Format(periodLayout))
	query.Set("securityToken", c.token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, c.redact(err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, c.redact(err)
	}
	defer func() { _ = resp.Body.Close() }()

	series, err := Decode(io.LimitReader(resp.Body, maxBody))
	if ack := (*AcknowledgementError)(nil); errors.As(err, &ack) {
		ack.StatusCode = resp.StatusCode

		return nil, ack
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{StatusCode: resp.StatusCode}
	}

	return series, err
}

// redact drops the request's URL, which holds the token, from err.
func (c *Client) redact(err error) error {
	if ue := (*url.Error)(nil); errors.As(err, &ue) {
		return &url.Error{Op: ue.Op, URL: c.baseURL, Err: ue.Err}
	}

	return err
}

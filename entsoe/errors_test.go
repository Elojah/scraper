package entsoe_test

import (
	"testing"

	"github.com/elojah/scraper/entsoe"
)

func TestError(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		err  error
		want string
	}{
		{&entsoe.AcknowledgementError{Code: "999", Text: "Authentication failed."}, "entsoe: refused (999): Authentication failed."},
		{&entsoe.StatusError{StatusCode: 429}, "entsoe: HTTP 429 Too Many Requests"},
		{&entsoe.DocumentError{Reason: "no point"}, "entsoe: invalid document: no point"},
	} {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%q, want %q", got, c.want)
		}
	}
}

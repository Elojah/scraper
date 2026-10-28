package entsoe

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	// ErrNoData is matched when nothing of the range asked is published, such as tomorrow's prices before about
	// 12:45 CET.
	ErrNoData = errors.New("entsoe: no data")
	// ErrUnauthorized is matched when ENTSO-E refuses the security token.
	ErrUnauthorized = errors.New("entsoe: unauthorized")
	// ErrRateLimited is matched when ENTSO-E refuses more requests for now: it takes 400 a minute per token.
	ErrRateLimited = errors.New("entsoe: rate limited")
	// ErrInvalidDocument is matched by a *DocumentError.
	ErrInvalidDocument = errors.New("entsoe: invalid document")
)

// noData starts the text of the acknowledgement of a range with nothing published.
const noData = "No matching data found"

// AcknowledgementError is a request ENTSO-E refused, with an Acknowledgement_MarketDocument's reason. It matches
// ErrNoData when nothing is published, ErrUnauthorized and ErrRateLimited by its status.
type AcknowledgementError struct {
	// StatusCode is the HTTP status it came with, 0 out of Decode.
	StatusCode int
	// Code is the reason's, 999 whatever the reason.
	Code string
	Text string
}

func (e *AcknowledgementError) Error() string {
	return fmt.Sprintf("entsoe: refused (%s): %s", e.Code, e.Text)
}

func (e *AcknowledgementError) Is(target error) bool {
	if target == ErrNoData {
		return strings.HasPrefix(e.Text, noData)
	}
	if target == ErrUnauthorized {
		return e.StatusCode == http.StatusUnauthorized
	}

	return target == ErrRateLimited && e.StatusCode == http.StatusTooManyRequests
}

// StatusError is an HTTP status other than 200 that came without an acknowledgement. It matches ErrUnauthorized and
// ErrRateLimited by its status.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("entsoe: HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

func (e *StatusError) Is(target error) bool {
	if target == ErrUnauthorized {
		return e.StatusCode == http.StatusUnauthorized
	}

	return target == ErrRateLimited && e.StatusCode == http.StatusTooManyRequests
}

// DocumentError is an answer that is not a document Decode reads. It matches ErrInvalidDocument.
type DocumentError struct {
	Reason string
}

func (e *DocumentError) Error() string {
	return "entsoe: invalid document: " + e.Reason
}

func (e *DocumentError) Is(target error) bool {
	return target == ErrInvalidDocument
}

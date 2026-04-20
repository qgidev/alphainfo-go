package alphainfo

import (
	"errors"
	"fmt"
)

// Error is the base error type. Use errors.Is with the sentinel values
// below to discriminate instead of type assertions.
type Error struct {
	Message      string
	StatusCode   int
	Kind         error // One of the ErrXxx sentinels below; use errors.Is.
	ResponseData map[string]interface{}
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("alphainfo: %s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("alphainfo: %s", e.Message)
}

// Is implements errors.Is-compatible matching against the sentinels.
func (e *Error) Is(target error) bool {
	if e.Kind == nil {
		return false
	}
	return errors.Is(e.Kind, target)
}

// Sentinel errors — use errors.Is for discrimination.
//
//	if errors.Is(err, alphainfo.ErrAuth) { ... }
var (
	// ErrAuth is returned for HTTP 401 — invalid or missing API key.
	// Not retryable. Get a key at https://alphainfo.io/register.
	ErrAuth = errors.New("auth error")

	// ErrRateLimit is returned for HTTP 429. The returned *Error embeds
	// RetryAfter (seconds) when the server provides it.
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrValidation is returned for HTTP 400, 413, 422. Not retryable.
	ErrValidation = errors.New("validation error")

	// ErrNotFound is returned for HTTP 404.
	ErrNotFound = errors.New("not found")

	// ErrAPI is returned for HTTP 5xx.
	ErrAPI = errors.New("server error")

	// ErrNetwork is returned for transport-level failures (DNS, TCP,
	// TLS, timeouts, context cancellation).
	ErrNetwork = errors.New("network error")
)

// RateLimitError is returned for HTTP 429 responses. Fetch the
// server's Retry-After hint (in seconds) via the field of the same
// name; use errors.As to get at it:
//
//	var rl *alphainfo.RateLimitError
//	if errors.As(err, &rl) {
//	    time.Sleep(time.Duration(rl.RetryAfter) * time.Second)
//	}
type RateLimitError struct {
	Base       *Error // underlying error with message + status code.
	RetryAfter int    // seconds; 0 if the server did not provide a hint.
}

func (e *RateLimitError) Error() string {
	base := e.Base.Error()
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s (retry after %ds)", base, e.RetryAfter)
	}
	return base
}

// StatusCode exposes the HTTP status code of the original response.
func (e *RateLimitError) StatusCode() int { return e.Base.StatusCode }

// Unwrap lets errors.Is traverse to the wrapped *Error / sentinel.
func (e *RateLimitError) Unwrap() error { return e.Base }

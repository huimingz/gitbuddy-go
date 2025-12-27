package llm

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrorType represents the classification of an error for retry purposes
type ErrorType int

const (
	// ErrorTypeRetryable indicates the error is transient and can be retried
	ErrorTypeRetryable ErrorType = iota
	// ErrorTypeNonRetryable indicates the error is permanent and should not be retried
	ErrorTypeNonRetryable
	// ErrorTypeUnknown indicates the error type is unknown (conservative: don't retry)
	ErrorTypeUnknown
)

// String returns the string representation of ErrorType
func (e ErrorType) String() string {
	switch e {
	case ErrorTypeRetryable:
		return "Retryable"
	case ErrorTypeNonRetryable:
		return "NonRetryable"
	case ErrorTypeUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// HTTPStatusError is an interface for errors that have HTTP status codes
type HTTPStatusError interface {
	error
	HTTPStatusCode() int
}

// ClassifyError determines if an error is retryable based on its type and content
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeNonRetryable
	}

	// Check for context cancellation (user interrupted)
	if errors.Is(err, context.Canceled) {
		return ErrorTypeNonRetryable
	}

	// Check for context deadline exceeded (timeout - retryable)
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTypeRetryable
	}

	// Check for network errors
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return ErrorTypeRetryable
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrorTypeRetryable
	}

	// Check for HTTP status errors
	if statusErr, ok := err.(HTTPStatusError); ok {
		return classifyHTTPStatus(statusErr.HTTPStatusCode())
	}

	// Check for HTTPError type (used in tests)
	type httpError interface {
		error
		StatusCode() int
	}
	if httpErr, ok := err.(httpError); ok {
		return classifyHTTPStatus(httpErr.StatusCode())
	}

	// Check error message for context length issues
	errMsg := strings.ToLower(err.Error())
	contextKeywords := []string{
		"context length",
		"context_length",
		"maximum context",
		"token limit",
		"tokens exceeded",
	}
	for _, keyword := range contextKeywords {
		if strings.Contains(errMsg, keyword) {
			return ErrorTypeNonRetryable
		}
	}

	// Check for timeout in error message
	if strings.Contains(errMsg, "timeout") {
		return ErrorTypeRetryable
	}

	// Conservative approach: unknown errors are not retried
	return ErrorTypeUnknown
}

// classifyHTTPStatus classifies HTTP status codes
func classifyHTTPStatus(statusCode int) ErrorType {
	switch statusCode {
	case http.StatusTooManyRequests: // 429
		return ErrorTypeRetryable
	case http.StatusServiceUnavailable: // 503
		return ErrorTypeRetryable
	case http.StatusBadGateway: // 502
		return ErrorTypeRetryable
	case http.StatusGatewayTimeout: // 504
		return ErrorTypeRetryable
	case http.StatusBadRequest: // 400
		return ErrorTypeNonRetryable
	case http.StatusUnauthorized: // 401
		return ErrorTypeNonRetryable
	case http.StatusForbidden: // 403
		return ErrorTypeNonRetryable
	case http.StatusNotFound: // 404
		return ErrorTypeNonRetryable
	default:
		if statusCode >= 500 {
			return ErrorTypeRetryable // Server errors are generally retryable
		}
		if statusCode >= 400 {
			return ErrorTypeNonRetryable // Client errors are not retryable
		}
		return ErrorTypeUnknown
	}
}

// CalculateBackoff calculates the backoff duration for a retry attempt using exponential backoff
// Formula: min(base * 2^(attempt-1), max)
func CalculateBackoff(attempt int, base, max float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	// Calculate exponential backoff: base * 2^(attempt-1)
	backoff := base * math.Pow(2, float64(attempt-1))

	// Cap at maximum
	if backoff > max {
		backoff = max
	}

	return time.Duration(backoff * float64(time.Second))
}

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	Enabled     bool    // Whether retry is enabled
	MaxAttempts int     // Maximum number of retry attempts
	BackoffBase float64 // Base backoff duration in seconds
	BackoffMax  float64 // Maximum backoff duration in seconds
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		BackoffBase: 1.0,
		BackoffMax:  8.0,
	}
}

// Validate validates the retry configuration
func (c *RetryConfig) Validate() error {
	if c.MaxAttempts < 0 {
		return errors.New("max_attempts must be non-negative")
	}
	if c.BackoffBase < 0 {
		return errors.New("backoff_base must be non-negative")
	}
	if c.BackoffMax < c.BackoffBase {
		return errors.New("backoff_max must be greater than or equal to backoff_base")
	}
	return nil
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// WithRetry executes a function with retry logic
func WithRetry(ctx context.Context, cfg RetryConfig, fn RetryableFunc) error {
	if !cfg.Enabled || cfg.MaxAttempts <= 0 {
		// Retry disabled, execute once
		return fn()
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts+1; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Classify error
		errType := ClassifyError(err)

		// Don't retry non-retryable or unknown errors
		if errType != ErrorTypeRetryable {
			return err
		}

		// Don't retry if this was the last attempt
		if attempt > cfg.MaxAttempts {
			return err
		}

		// Calculate backoff and wait
		backoff := CalculateBackoff(attempt, cfg.BackoffBase, cfg.BackoffMax)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return lastErr
}

// RetryableFuncWithResult is a function that can be retried and returns a result
type RetryableFuncWithResult[T any] func() (T, error)

// WithRetryResult executes a function with retry logic and returns a result
func WithRetryResult[T any](ctx context.Context, cfg RetryConfig, fn RetryableFuncWithResult[T]) (T, error) {
	var zero T

	if !cfg.Enabled || cfg.MaxAttempts <= 0 {
		// Retry disabled, execute once
		return fn()
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts+1; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Execute function
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Classify error
		errType := ClassifyError(err)

		// Don't retry non-retryable or unknown errors
		if errType != ErrorTypeRetryable {
			return zero, err
		}

		// Don't retry if this was the last attempt
		if attempt > cfg.MaxAttempts {
			return zero, err
		}

		// Calculate backoff and wait
		backoff := CalculateBackoff(attempt, cfg.BackoffBase, cfg.BackoffMax)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return zero, lastErr
}

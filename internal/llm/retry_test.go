package llm

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestClassifyError_NetworkErrors tests network error classification
func TestClassifyError_NetworkErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{
			name: "network timeout",
			err:  &net.OpError{Op: "dial", Err: errors.New("timeout")},
			want: ErrorTypeRetryable,
		},
		{
			name: "connection refused",
			err:  &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			want: ErrorTypeRetryable,
		},
		{
			name: "DNS error",
			err:  &net.DNSError{Err: "no such host"},
			want: ErrorTypeRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClassifyError_TimeoutErrors tests timeout error classification
func TestClassifyError_TimeoutErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: ErrorTypeRetryable,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: ErrorTypeNonRetryable, // User cancellation should not retry
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func (e *HTTPError) StatusCode() int {
	return e.Code
}

// TestClassifyError_HTTP503 tests 503 error classification
func TestClassifyError_HTTP503(t *testing.T) {
	err := &HTTPError{Code: http.StatusServiceUnavailable, Message: "service unavailable"}
	got := ClassifyError(err)
	if got != ErrorTypeRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeRetryable)
	}
}

// TestClassifyError_HTTP429 tests 429 error classification
func TestClassifyError_HTTP429(t *testing.T) {
	err := &HTTPError{Code: http.StatusTooManyRequests, Message: "too many requests"}
	got := ClassifyError(err)
	if got != ErrorTypeRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeRetryable)
	}
}

// TestClassifyError_HTTP400 tests 400 error classification
func TestClassifyError_HTTP400(t *testing.T) {
	err := &HTTPError{Code: http.StatusBadRequest, Message: "bad request"}
	got := ClassifyError(err)
	if got != ErrorTypeNonRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeNonRetryable)
	}
}

// TestClassifyError_HTTP401 tests 401 error classification
func TestClassifyError_HTTP401(t *testing.T) {
	err := &HTTPError{Code: http.StatusUnauthorized, Message: "unauthorized"}
	got := ClassifyError(err)
	if got != ErrorTypeNonRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeNonRetryable)
	}
}

// TestClassifyError_ContextExceeded tests context exceeded error classification
func TestClassifyError_ContextExceeded(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{
			name: "context length exceeded",
			err:  errors.New("context length exceeded"),
			want: ErrorTypeNonRetryable,
		},
		{
			name: "maximum context length",
			err:  errors.New("maximum context length is 128000"),
			want: ErrorTypeNonRetryable,
		},
		{
			name: "token limit exceeded",
			err:  errors.New("token limit exceeded"),
			want: ErrorTypeNonRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClassifyError_UnknownError tests unknown error classification
func TestClassifyError_UnknownError(t *testing.T) {
	err := errors.New("some unknown error")
	got := ClassifyError(err)
	if got != ErrorTypeUnknown {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeUnknown)
	}
}

// TestCalculateBackoff_FirstAttempt tests first retry backoff
func TestCalculateBackoff_FirstAttempt(t *testing.T) {
	got := CalculateBackoff(1, 1.0, 8.0)
	want := 1 * time.Second
	if got != want {
		t.Errorf("CalculateBackoff(1) = %v, want %v", got, want)
	}
}

// TestCalculateBackoff_SecondAttempt tests second retry backoff
func TestCalculateBackoff_SecondAttempt(t *testing.T) {
	got := CalculateBackoff(2, 1.0, 8.0)
	want := 2 * time.Second
	if got != want {
		t.Errorf("CalculateBackoff(2) = %v, want %v", got, want)
	}
}

// TestCalculateBackoff_ThirdAttempt tests third retry backoff
func TestCalculateBackoff_ThirdAttempt(t *testing.T) {
	got := CalculateBackoff(3, 1.0, 8.0)
	want := 4 * time.Second
	if got != want {
		t.Errorf("CalculateBackoff(3) = %v, want %v", got, want)
	}
}

// TestCalculateBackoff_MaxBackoff tests maximum backoff limit
func TestCalculateBackoff_MaxBackoff(t *testing.T) {
	got := CalculateBackoff(10, 1.0, 8.0)
	want := 8 * time.Second
	if got != want {
		t.Errorf("CalculateBackoff(10) = %v, want %v (should be capped at max)", got, want)
	}
}

// TestCalculateBackoff_CustomBase tests custom base backoff
func TestCalculateBackoff_CustomBase(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		base    float64
		max     float64
		want    time.Duration
	}{
		{
			name:    "base 2.0, attempt 1",
			attempt: 1,
			base:    2.0,
			max:     16.0,
			want:    2 * time.Second,
		},
		{
			name:    "base 2.0, attempt 2",
			attempt: 2,
			base:    2.0,
			max:     16.0,
			want:    4 * time.Second,
		},
		{
			name:    "base 0.5, attempt 1",
			attempt: 1,
			base:    0.5,
			max:     8.0,
			want:    500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBackoff(tt.attempt, tt.base, tt.max)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d, %v, %v) = %v, want %v", tt.attempt, tt.base, tt.max, got, tt.want)
			}
		})
	}
}

// TestErrorType_String tests ErrorType string representation
func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name string
		et   ErrorType
		want string
	}{
		{"Retryable", ErrorTypeRetryable, "Retryable"},
		{"NonRetryable", ErrorTypeNonRetryable, "NonRetryable"},
		{"Unknown", ErrorTypeUnknown, "Unknown"},
		{"Invalid", ErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.et.String()
			if got != tt.want {
				t.Errorf("ErrorType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClassifyError_NilError tests nil error classification
func TestClassifyError_NilError(t *testing.T) {
	got := ClassifyError(nil)
	if got != ErrorTypeNonRetryable {
		t.Errorf("ClassifyError(nil) = %v, want %v", got, ErrorTypeNonRetryable)
	}
}

// TestClassifyError_HTTP502 tests 502 error classification
func TestClassifyError_HTTP502(t *testing.T) {
	err := &HTTPError{Code: http.StatusBadGateway, Message: "bad gateway"}
	got := ClassifyError(err)
	if got != ErrorTypeRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeRetryable)
	}
}

// TestClassifyError_HTTP504 tests 504 error classification
func TestClassifyError_HTTP504(t *testing.T) {
	err := &HTTPError{Code: http.StatusGatewayTimeout, Message: "gateway timeout"}
	got := ClassifyError(err)
	if got != ErrorTypeRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeRetryable)
	}
}

// TestClassifyError_HTTP403 tests 403 error classification
func TestClassifyError_HTTP403(t *testing.T) {
	err := &HTTPError{Code: http.StatusForbidden, Message: "forbidden"}
	got := ClassifyError(err)
	if got != ErrorTypeNonRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeNonRetryable)
	}
}

// TestClassifyError_HTTP404 tests 404 error classification
func TestClassifyError_HTTP404(t *testing.T) {
	err := &HTTPError{Code: http.StatusNotFound, Message: "not found"}
	got := ClassifyError(err)
	if got != ErrorTypeNonRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeNonRetryable)
	}
}

// TestClassifyError_HTTP500 tests 500 error classification
func TestClassifyError_HTTP500(t *testing.T) {
	err := &HTTPError{Code: http.StatusInternalServerError, Message: "internal server error"}
	got := ClassifyError(err)
	if got != ErrorTypeRetryable {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeRetryable)
	}
}

// TestClassifyError_HTTP200 tests 200 success classification
func TestClassifyError_HTTP200(t *testing.T) {
	err := &HTTPError{Code: http.StatusOK, Message: "ok"}
	got := ClassifyError(err)
	if got != ErrorTypeUnknown {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrorTypeUnknown)
	}
}

// TestDefaultRetryConfig tests default retry configuration
func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if !cfg.Enabled {
		t.Error("DefaultRetryConfig().Enabled should be true")
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("DefaultRetryConfig().MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 1.0 {
		t.Errorf("DefaultRetryConfig().BackoffBase = %f, want 1.0", cfg.BackoffBase)
	}
	if cfg.BackoffMax != 8.0 {
		t.Errorf("DefaultRetryConfig().BackoffMax = %f, want 8.0", cfg.BackoffMax)
	}
}

// TestRetryConfig_Validate tests retry configuration validation
func TestRetryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RetryConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultRetryConfig(),
			wantErr: false,
		},
		{
			name: "negative max attempts",
			config: RetryConfig{
				MaxAttempts: -1,
				BackoffBase: 1.0,
				BackoffMax:  8.0,
			},
			wantErr: true,
		},
		{
			name: "negative backoff base",
			config: RetryConfig{
				MaxAttempts: 3,
				BackoffBase: -1.0,
				BackoffMax:  8.0,
			},
			wantErr: true,
		},
		{
			name: "backoff max less than base",
			config: RetryConfig{
				MaxAttempts: 3,
				BackoffBase: 10.0,
				BackoffMax:  5.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCalculateBackoff_ZeroAttempt tests backoff with zero attempt
func TestCalculateBackoff_ZeroAttempt(t *testing.T) {
	got := CalculateBackoff(0, 1.0, 8.0)
	want := 1 * time.Second // Should treat 0 as 1
	if got != want {
		t.Errorf("CalculateBackoff(0) = %v, want %v", got, want)
	}
}

// TestWithRetry_Success tests successful execution without retry
func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := WithRetry(ctx, cfg, fn)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("Function called %d times, want 1", callCount)
	}
}

// TestWithRetry_RetryableError tests retry on retryable error
func TestWithRetry_RetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		BackoffBase: 0.01, // Short backoff for testing
		BackoffMax:  0.1,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &HTTPError{Code: http.StatusServiceUnavailable, Message: "service unavailable"}
		}
		return nil
	}

	err := WithRetry(ctx, cfg, fn)
	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}
	if callCount != 3 {
		t.Errorf("Function called %d times, want 3", callCount)
	}
}

// TestWithRetry_NonRetryableError tests no retry on non-retryable error
func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultRetryConfig()

	callCount := 0
	fn := func() error {
		callCount++
		return &HTTPError{Code: http.StatusBadRequest, Message: "bad request"}
	}

	err := WithRetry(ctx, cfg, fn)
	if err == nil {
		t.Error("WithRetry() should return error")
	}
	if callCount != 1 {
		t.Errorf("Function called %d times, want 1 (should not retry)", callCount)
	}
}

// TestWithRetry_MaxAttemptsExceeded tests max attempts limit
func TestWithRetry_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		Enabled:     true,
		MaxAttempts: 2,
		BackoffBase: 0.01,
		BackoffMax:  0.1,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return &HTTPError{Code: http.StatusServiceUnavailable, Message: "service unavailable"}
	}

	err := WithRetry(ctx, cfg, fn)
	if err == nil {
		t.Error("WithRetry() should return error after max attempts")
	}
	if callCount != 3 { // 1 initial + 2 retries
		t.Errorf("Function called %d times, want 3", callCount)
	}
}

// TestWithRetry_ContextCanceled tests context cancellation
func TestWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := DefaultRetryConfig()

	callCount := 0
	fn := func() error {
		callCount++
		return &HTTPError{Code: http.StatusServiceUnavailable, Message: "service unavailable"}
	}

	err := WithRetry(ctx, cfg, fn)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("WithRetry() error = %v, want context.Canceled", err)
	}
}

// TestWithRetry_Disabled tests retry disabled
func TestWithRetry_Disabled(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		Enabled:     false,
		MaxAttempts: 3,
		BackoffBase: 1.0,
		BackoffMax:  8.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return &HTTPError{Code: http.StatusServiceUnavailable, Message: "service unavailable"}
	}

	err := WithRetry(ctx, cfg, fn)
	if err == nil {
		t.Error("WithRetry() should return error when retry disabled")
	}
	if callCount != 1 {
		t.Errorf("Function called %d times, want 1 (retry disabled)", callCount)
	}
}

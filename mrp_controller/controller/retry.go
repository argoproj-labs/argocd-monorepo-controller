package controller

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// Default retry policy
const (
	DefaultRetryMaxAttempts   = 3
	DefaultRetryBaseDelay     = 20 * time.Second
	DefaultRetryMaxDelay      = 15 * time.Minute
	DefaultRetryBackoffFactor = 2.0
	DefaultRetryJitter        = 0.4
)

// RetryPolicy defines how an application whose handling failed with a
// retriable error is retried.
type RetryPolicy struct {
	// MaxAttempts is the number of retries of a single application event.
	MaxAttempts int
	// BaseDelay is the delay before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the delay between the retries. Not capped when <= 0
	MaxDelay time.Duration
	// Factor is the multiplier applied to the delay of every next retry.
	Factor float64
	// Jitter is the fraction by which the delay is randomly varied, so that
	// the applications which failed together are not all retried at once.
	Jitter float64
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: DefaultRetryMaxAttempts,
		BaseDelay:   DefaultRetryBaseDelay,
		MaxDelay:    DefaultRetryMaxDelay,
		Factor:      DefaultRetryBackoffFactor,
		Jitter:      DefaultRetryJitter,
	}
}

func (p RetryPolicy) Enabled() bool {
	return p.MaxAttempts > 0
}

func (p RetryPolicy) Validate() error {
	var errs []error
	if p.MaxAttempts < 0 {
		errs = append(errs, fmt.Errorf("max attempts is %d, must not be negative", p.MaxAttempts))
	}
	if p.BaseDelay <= 0 {
		errs = append(errs, fmt.Errorf("base delay is %v, must be positive", p.BaseDelay))
	}
	if p.MaxDelay < 0 {
		errs = append(errs, fmt.Errorf("max delay is %v, must not be negative", p.MaxDelay))
	}
	if p.MaxDelay > 0 && p.BaseDelay > p.MaxDelay {
		errs = append(errs, fmt.Errorf("base delay %v is longer than max delay %v", p.BaseDelay, p.MaxDelay))
	}
	if p.Factor < 1 {
		errs = append(errs, fmt.Errorf("backoff factor is %v, must be at least 1", p.Factor))
	}
	if p.Jitter < 0 || p.Jitter > 1 {
		errs = append(errs, fmt.Errorf("jitter is %v, must be between 0 and 1", p.Jitter))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid retry policy: %w", errors.Join(errs...))
}

// Backoff returns the delay before the given retry attempt, counted from 1.
// The delay grows exponentially with the number of the attempt, is never
// longer than MaxDelay, and is randomized by the Jitter fraction.
func (p RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := p.clampDelay(float64(p.BaseDelay) * math.Pow(p.Factor, float64(attempt-1)))
	if p.Jitter <= 0 {
		return delay
	}
	return p.clampDelay(float64(delay) * (1 + p.Jitter*(2*rand.Float64()-1)))
}

// clampDelay keeps the delay within MaxDelay
func (p RetryPolicy) clampDelay(delay float64) time.Duration {
	maxDelay := time.Duration(math.MaxInt64)
	if p.MaxDelay > 0 {
		maxDelay = p.MaxDelay
	}
	switch {
	case delay < 0:
		return 0
	case delay >= float64(maxDelay):
		return maxDelay
	default:
		return time.Duration(delay)
	}
}

// retryTimer is the handle of a retry which was scheduled but did not run yet.
type retryTimer interface {
	// Stop prevents the scheduled function from being run. It reports false
	// when the function already ran or was already stopped.
	Stop() bool
}

// timerFactory runs f after the delay d and returns the handle of the
// scheduled call.
type timerFactory func(d time.Duration, f func()) retryTimer

func realTimer(d time.Duration, f func()) retryTimer {
	return time.AfterFunc(d, f)
}

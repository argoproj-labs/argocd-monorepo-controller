package service

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// PermanentError marks an error as not worth retrying,
// other errors are treated as transient.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// NewPermanentError wraps err so that IsRetryable reports it as permanent.
// The wrapped error must not be a joined error, see IsRetryable.
func NewPermanentError(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsRetryable reports whether handling of an application may succeed when it
// is attempted again later. Errors of an unrecognized kind are reported as
// retryable, so that an unexpected failure is retried a bounded number of
// times instead of being dropped.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// A joined error reports the failures of several application sources, and
	// is retryable when retrying can help any one of them. This takes
	// precedence over the PermanentError marker, which would otherwise match a
	// single permanently failing source and suppress the retry for all of them.
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		for _, e := range joined.Unwrap() {
			if IsRetryable(e) {
				return true
			}
		}
		return false
	}

	var permanent *PermanentError
	if errors.As(err, &permanent) {
		return false
	}

	// The context of a single event is cancelled on shutdown
	if errors.Is(err, context.Canceled) {
		return false
	}
	// The context that expires on the event handling timeout. Worth retrying.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if retryable, recognized := isRetryableAPIError(err); recognized {
		return retryable
	}
	if retryable, recognized := isRetryableGRPCError(err); recognized {
		return retryable
	}
	return true
}

// isRetryableAPIError classifies errors returned by the Kubernetes API server.
// It reports recognized as false when err is not an API server error.
func isRetryableAPIError(err error) (retryable, recognized bool) {
	switch {
	case apierrors.IsConflict(err), apierrors.IsTooManyRequests(err), apierrors.IsServerTimeout(err),
		apierrors.IsTimeout(err), apierrors.IsInternalError(err), apierrors.IsServiceUnavailable(err):
		return true, true
	case apierrors.IsNotFound(err), apierrors.IsForbidden(err), apierrors.IsUnauthorized(err),
		apierrors.IsInvalid(err), apierrors.IsBadRequest(err), apierrors.IsMethodNotSupported(err),
		apierrors.IsGone(err), apierrors.IsRequestEntityTooLargeError(err):
		return false, true
	}
	return false, false
}

// isRetryableGRPCError classifies errors returned by the monorepo repo server.
// It reports recognized as false when err does not carry a gRPC status.
func isRetryableGRPCError(err error) (retryable, recognized bool) {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return false, false
	}
	switch grpcStatus.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted,
		codes.Aborted, codes.Internal, codes.Unknown:
		return true, true
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied,
		codes.FailedPrecondition, codes.OutOfRange, codes.Unimplemented, codes.Unauthenticated:
		return false, true
	}
	return false, false
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	grpcStatus, ok := status.FromError(err)
	if ok && grpcStatus.Code() == codes.DeadlineExceeded {
		return true
	}
	return false
}

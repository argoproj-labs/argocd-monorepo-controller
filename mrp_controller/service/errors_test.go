package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var appsResource = schema.GroupResource{Group: "argoproj.io", Resource: "applications"}

func TestPermanentError(t *testing.T) {
	cause := errors.New("manifest generation paths not set")
	err := NewPermanentError(cause)

	assert.Equal(t, cause.Error(), err.Error())
	require.ErrorIs(t, err, cause)
	// nothing is wrapped when there is no error to mark
	assert.NoError(t, NewPermanentError(nil))
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "no error",
			err:  nil,
			want: false,
		},
		{
			name: "error of an unknown kind is retried",
			err:  errors.New("something unexpected"),
			want: true,
		},
		{
			name: "permanent error",
			err:  NewPermanentError(errors.New("manifest generation paths not set")),
			want: false,
		},
		{
			name: "wrapped permanent error",
			err:  fmt.Errorf("failed to handle application: %w", NewPermanentError(errors.New("bad patch"))),
			want: false,
		},
		{
			name: "cancelled context, the controller is shutting down",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "wrapped cancelled context",
			err:  fmt.Errorf("error getting change revision: %w", context.Canceled),
			want: false,
		},
		{
			name: "expired context, the event handling timeout was reached",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "application does not exist",
			err:  apierrors.NewNotFound(appsResource, "test-app"),
			want: false,
		},
		{
			name: "wrapped application does not exist",
			err:  fmt.Errorf("error getting application: %w", apierrors.NewNotFound(appsResource, "test-app")),
			want: false,
		},
		{
			name: "no permission to patch the application",
			err:  apierrors.NewForbidden(appsResource, "test-app", errors.New("not allowed")),
			want: false,
		},
		{
			name: "conflicting application update",
			err:  apierrors.NewConflict(appsResource, "test-app", errors.New("object was modified")),
			want: true,
		},
		{
			name: "api server is overloaded",
			err:  apierrors.NewTooManyRequests("slow down", 1),
			want: true,
		},
		{
			name: "api server internal error",
			err:  apierrors.NewInternalError(errors.New("boom")),
			want: true,
		},
		{
			name: "repo server is unavailable",
			err:  status.Error(codes.Unavailable, "connection refused"),
			want: true,
		},
		{
			name: "wrapped unavailable repo server",
			err:  fmt.Errorf("error getting change revision: %w", status.Error(codes.Unavailable, "connection refused")),
			want: true,
		},
		{
			name: "repo server request timed out",
			err:  status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
			want: true,
		},
		{
			name: "manifest generation paths are not set",
			err:  status.Error(codes.FailedPrecondition, "manifest generation paths not set"),
			want: false,
		},
		{
			name: "repo server rejected the request",
			err:  status.Error(codes.InvalidArgument, "no such revision"),
			want: false,
		},
		{
			name: "joined source errors, one of them is retryable",
			err: errors.Join(
				NewPermanentError(errors.New("source 0: no manifest generation paths")),
				status.Error(codes.Unavailable, "source 1: connection refused"),
			),
			want: true,
		},
		{
			name: "joined source errors, all of them are permanent",
			err: errors.Join(
				NewPermanentError(errors.New("source 0: no manifest generation paths")),
				status.Error(codes.InvalidArgument, "source 1: no such revision"),
			),
			want: false,
		},
		{
			name: "joined single permanent source error",
			err:  errors.Join(NewPermanentError(errors.New("source 0: no manifest generation paths"))),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryable(tt.err))
		})
	}
}

func TestIsTimeout(t *testing.T) {
	assert.False(t, isTimeout(nil))
	assert.False(t, isTimeout(errors.New("something unexpected")))
	assert.True(t, isTimeout(context.DeadlineExceeded))
	assert.True(t, isTimeout(fmt.Errorf("error getting change revision: %w", context.DeadlineExceeded)))
	assert.True(t, isTimeout(status.Error(codes.DeadlineExceeded, "context deadline exceeded")))
	assert.False(t, isTimeout(status.Error(codes.Unavailable, "connection refused")))
}

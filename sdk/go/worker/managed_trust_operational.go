package worker

import (
	"context"
	"errors"
)

type managedTrustOperationalError struct{ cause error }

func (e *managedTrustOperationalError) Error() string { return e.cause.Error() }
func (e *managedTrustOperationalError) Unwrap() error { return e.cause }

func newManagedTrustOperationalError(cause error) error {
	return &managedTrustOperationalError{cause: cause}
}

func isManagedTrustOperationalError(err error) bool {
	var operational *managedTrustOperationalError
	return errors.As(err, &operational) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

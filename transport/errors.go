package transport

import "fmt"

// ErrResourceNotFound is returned for 404 responses from the Geni API.
var ErrResourceNotFound = fmt.Errorf("resource not found")

// ErrAccessDenied is returned for 403 responses from the Geni API.
var ErrAccessDenied = fmt.Errorf("access denied")

// errRetry signals that a request should be retried (429, 401, or
// transient transport errors). The retry-go RetryIf hook matches on
// this concrete type via errors.As.
type errRetry struct {
	statusCode        int
	secondsUntilRetry int
}

func (e errRetry) Error() string {
	return fmt.Sprintf("received %d status, retry in %d seconds", e.statusCode, e.secondsUntilRetry)
}

func newErrRetry(statusCode int, secondsUntilRetry int) error {
	return errRetry{statusCode: statusCode, secondsUntilRetry: secondsUntilRetry}
}

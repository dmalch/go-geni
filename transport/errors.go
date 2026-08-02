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

// errIncapsula signals that Incapsula, the DDoS protection service in front
// of Geni, blocked the request. It is retryable — but on its own terms, not
// the errRetry ladder's: a bot-protection block does not clear in the two to
// four seconds a 429 retry waits, so it gets a much longer delay
// (incapsulaRetryDelay) and a far smaller attempt budget
// (maxIncapsulaRetries), because retrying into a block risks prolonging it.
//
// Kept a distinct type rather than an errRetry with a big secondsUntilRetry:
// the delay and the attempt cap both key off the type, and the message must
// stay free of a "retry in N seconds" figure Incapsula never gave us.
type errIncapsula struct{}

func (errIncapsula) Error() string {
	return "incapsula blocked request"
}

package domain

import "errors"

// Sentinel errors for cross-provider error classification.
// Providers should wrap these so the CLI can handle error categories
// uniformly without importing provider-specific SDKs.
//
//	return fmt.Errorf("failed to delete server: %w", domain.ErrNotFound)
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrUnauthorized indicates the request was rejected due to
	// invalid, expired, or missing credentials.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrRateLimited indicates the provider throttled the request.
	ErrRateLimited = errors.New("rate limited")

	// ErrConflict indicates a state or uniqueness conflict, such as
	// a duplicate server name or an operation on a server in a
	// transitional state.
	ErrConflict = errors.New("conflict")
)

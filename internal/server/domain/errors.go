package domain

import shared "nathanbeddoewebdev/vpsm/internal/domain"

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = shared.ErrNotFound
	// ErrUnauthorized indicates the request was rejected due to
	// invalid, expired, or missing credentials.
	ErrUnauthorized = shared.ErrUnauthorized
	// ErrRateLimited indicates the provider throttled the request.
	ErrRateLimited = shared.ErrRateLimited
	// ErrConflict indicates a state or uniqueness conflict.
	ErrConflict = shared.ErrConflict
)

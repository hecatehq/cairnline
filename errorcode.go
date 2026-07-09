package cairnline

import (
	"errors"

	"github.com/hecatehq/cairnline/internal/core"
)

// Tool-error codes are the stable, machine-readable contract a host places on
// MCP tool failures. Each code names one class of failure so a host can map it
// to an HTTP status (or its own error taxonomy) without parsing human prose.
//
// The codes derive from the four typed store sentinels plus one default. They
// are strings rather than an enum so the wire contract stays trivially JSON-
// encodable and forward-compatible for non-Go clients.
const (
	// ErrorCodeNotFound reports that a referenced entity does not exist.
	// Suggested host HTTP status: 404.
	ErrorCodeNotFound = "not_found"
	// ErrorCodeInvalid reports bad or missing input, including argument-decode
	// failures and domain validation errors. Suggested host HTTP status: 400.
	ErrorCodeInvalid = "invalid"
	// ErrorCodeAlreadyExists reports an id or uniqueness collision.
	// Suggested host HTTP status: 409.
	ErrorCodeAlreadyExists = "already_exists"
	// ErrorCodeConflict reports an invalid state transition or a claim race —
	// the request was well-formed but conflicts with current state.
	// Suggested host HTTP status: 409.
	ErrorCodeConflict = "conflict"
	// ErrorCodeInternal is the default for any unexpected, unclassified
	// server-side failure. Suggested host HTTP status: 500.
	ErrorCodeInternal = "internal"
)

// ClassifyErrorCode maps an error to its stable tool-error code by matching the
// typed store sentinels with errors.Is, so wrapped and joined errors classify
// correctly. It returns "" for a nil error and ErrorCodeInternal for anything
// that matches no sentinel.
//
// The match order is deliberate: NotFound, then Duplicate (already_exists),
// then Conflict, then Invalid. Duplicate is checked before Invalid because a
// uniqueness collision is often reported alongside a validation wrapper, and
// the more specific "already_exists" is the useful signal for a host.
func ClassifyErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, core.ErrNotFound):
		return ErrorCodeNotFound
	case errors.Is(err, core.ErrDuplicate):
		return ErrorCodeAlreadyExists
	case errors.Is(err, core.ErrConflict):
		return ErrorCodeConflict
	case errors.Is(err, core.ErrInvalid):
		return ErrorCodeInvalid
	default:
		return ErrorCodeInternal
	}
}

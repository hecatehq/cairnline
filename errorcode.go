package cairnline

import "github.com/hecatehq/cairnline/internal/core"

// Tool-error codes are the stable, machine-readable contract a host places on
// MCP tool failures. Each code names one class of failure so a host can map it
// to an HTTP status (or its own error taxonomy) without parsing human prose.
//
// The canonical definitions live in internal/core beside the store sentinels so
// the MCP server can classify errors without importing the root package (which
// would form an import cycle). These names are the public surface re-exported
// for external callers of the cairnline module.
const (
	// ErrorCodeNotFound reports that a referenced entity does not exist.
	// Suggested host HTTP status: 404.
	ErrorCodeNotFound = core.ErrorCodeNotFound
	// ErrorCodeInvalid reports bad or missing input, including argument-decode
	// failures and domain validation errors. Suggested host HTTP status: 400.
	ErrorCodeInvalid = core.ErrorCodeInvalid
	// ErrorCodeAlreadyExists reports an id or uniqueness collision.
	// Suggested host HTTP status: 409.
	ErrorCodeAlreadyExists = core.ErrorCodeAlreadyExists
	// ErrorCodeConflict reports an invalid state transition or a claim race —
	// the request was well-formed but conflicts with current state.
	// Suggested host HTTP status: 409.
	ErrorCodeConflict = core.ErrorCodeConflict
	// ErrorCodeInternal is the default for any unexpected, unclassified
	// server-side failure. Suggested host HTTP status: 500.
	ErrorCodeInternal = core.ErrorCodeInternal
)

// ClassifyErrorCode maps an error to its stable tool-error code by matching the
// typed store sentinels with errors.Is, so wrapped and joined errors classify
// correctly. It returns "" for a nil error and ErrorCodeInternal for anything
// that matches no sentinel. It delegates to internal/core, the canonical home
// of the classification logic.
func ClassifyErrorCode(err error) string {
	return core.ClassifyErrorCode(err)
}

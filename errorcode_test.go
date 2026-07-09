package cairnline_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hecatehq/cairnline"
)

func TestClassifyErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil error", err: nil, want: ""},
		{name: "not found", err: cairnline.ErrNotFound, want: cairnline.ErrorCodeNotFound},
		{name: "wrapped not found", err: fmt.Errorf("root: %w", cairnline.ErrNotFound), want: cairnline.ErrorCodeNotFound},
		{name: "duplicate is already_exists", err: errors.Join(cairnline.ErrDuplicate, errors.New("dup")), want: cairnline.ErrorCodeAlreadyExists},
		{name: "conflict", err: errors.Join(cairnline.ErrConflict, errors.New("race")), want: cairnline.ErrorCodeConflict},
		{name: "invalid", err: errors.Join(cairnline.ErrInvalid, errors.New("bad input")), want: cairnline.ErrorCodeInvalid},
		{name: "unclassified is internal", err: errors.New("boom"), want: cairnline.ErrorCodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cairnline.ClassifyErrorCode(tc.err); got != tc.want {
				t.Fatalf("ClassifyErrorCode(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

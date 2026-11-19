package boolean

import (
	"errors"
	"fmt"
)

// The errors of section 9. Each is wrapped with the stage and the operand,
// face or edge involved, and is matched with errors.Is.
var (
	// ErrInvalid reports a nil context, an unknown operation, invalid
	// options, non-finite input or an edge that does not lie on its faces.
	ErrInvalid = errors.New("boolean: invalid input")
	// ErrUnsupported reports an operand or a pair of carriers outside the
	// supported class.
	ErrUnsupported = errors.New("boolean: unsupported input")
	// ErrNonManifold reports a result whose boundary is not a manifold.
	ErrNonManifold = errors.New("boolean: result is not a manifold")
	// ErrTolerance reports a configuration that is neither coincident nor
	// separated at the working tolerance.
	ErrTolerance = errors.New("boolean: not resolved at the working tolerance")
	// ErrInconsistent reports an arrangement that contradicts itself.
	ErrInconsistent = errors.New("boolean: inconsistent arrangement")
	// ErrValidation reports a result that failed a check before return.
	ErrValidation = errors.New("boolean: result failed validation")
	// ErrBudget reports an exceeded work limit or integration budget.
	ErrBudget = errors.New("boolean: work limit exceeded")
)

// fail wraps one of the errors above with the stage that raised it.
func fail(kind error, stage, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", kind, stage, fmt.Sprintf(format, args...))
}

// failWith wraps one of the errors above together with its cause.
func failWith(kind error, stage string, cause error) error {
	return fmt.Errorf("%w: %s: %w", kind, stage, cause)
}

// operandName is the letter the specification gives an operand.
func operandName(index int) string { return string(rune('A' + index)) }

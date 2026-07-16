package reconcilers

import "errors"

// TerminalError marks a permanent configuration problem. The affected
// sub-reconciler should stop its own work, but the main reconcile chain
// should continue so unrelated maintenance (scaler, upgrade, restart, etc.)
// is not frozen until the operator restarts or the CR is deleted.
type TerminalError struct {
	Err error
}

func (e *TerminalError) Error() string {
	if e == nil || e.Err == nil {
		return "terminal error"
	}
	return e.Err.Error()
}

func (e *TerminalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AsTerminal wraps err as a TerminalError. A nil err is returned unchanged.
func AsTerminal(err error) error {
	if err == nil {
		return nil
	}
	var existing *TerminalError
	if errors.As(err, &existing) {
		return err
	}
	return &TerminalError{Err: err}
}

// IsTerminal reports whether err is or wraps a TerminalError.
func IsTerminal(err error) bool {
	var terminal *TerminalError
	return errors.As(err, &terminal)
}

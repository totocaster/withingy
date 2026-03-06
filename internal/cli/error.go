package cli

// ExitCoder is implemented by errors that want to control the process exit code.
type ExitCoder interface {
	error
	ExitCode() int
}

type exitError struct {
	err  error
	code int
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func (e *exitError) ExitCode() int {
	if e.code == 0 {
		return 1
	}
	return e.code
}

// NewExitError wraps err with an exit code. If err is nil a default message is used.
func NewExitError(code int, err error) error {
	if err == nil {
		return &exitError{err: nil, code: code}
	}
	return &exitError{err: err, code: code}
}

package main


// CodeStyleError is raised when code formatting fails style checks.
type CodeStyleError struct {
    msg string
}

func (e CodeStyleError) Error() string {
    return e.msg
}

// FormatError is raised when the format is incorrect.
type FormatError struct {
    msg string
}

func (e FormatError) Error() string {
    return e.msg
}

// MissingDependencyError is raised when an artifact references another artifact that is undefined.
type MissingDependencyError struct {
    msg string
}

func (e MissingDependencyError) Error() string {
    return e.msg
}

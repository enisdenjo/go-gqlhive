package gqlhive

import "log"

type Logger interface {
	Printf(format string, v ...any)
}

// NewLogger creates a new Logger instance that writes to the standard log output
// with the prefix "[gqlhive] " and includes the standard logging attribute such as timestamp.
func NewLogger() Logger {
	return log.New(log.Writer(), "[gqlhive] ", log.LstdFlags|log.Lmsgprefix)
}

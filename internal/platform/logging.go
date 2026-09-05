// Package platform contains process-level infrastructure.
package platform

import (
	"io"
	"log/slog"
)

// NewLogger returns the JSON logger used by the blog process.
func NewLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, nil))
}

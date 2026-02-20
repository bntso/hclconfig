package hclconfig

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// CycleError is returned when circular dependencies are detected between blocks.
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("circular dependency detected: %s", strings.Join(e.Cycle, " -> "))
}

// DiagnosticsError wraps HCL diagnostics as a Go error.
type DiagnosticsError struct {
	Diags hcl.Diagnostics
}

func (e *DiagnosticsError) Error() string {
	var msgs []string
	for _, d := range e.Diags {
		msgs = append(msgs, d.Summary)
	}
	return fmt.Sprintf("HCL diagnostics: %s", strings.Join(msgs, "; "))
}

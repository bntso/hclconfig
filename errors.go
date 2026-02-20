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
		var msg strings.Builder
		if d.Subject != nil {
			fmt.Fprintf(&msg, "%s:%d,%d: ", d.Subject.Filename, d.Subject.Start.Line, d.Subject.Start.Column)
		}
		msg.WriteString(d.Summary)
		if d.Detail != "" {
			msg.WriteString(": ")
			msg.WriteString(d.Detail)
		}
		msgs = append(msgs, msg.String())
	}
	return strings.Join(msgs, "\n")
}

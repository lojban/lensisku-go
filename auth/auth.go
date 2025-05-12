// Package auth contains authentication and authorization logic.
// This specific file, `auth.go`, seems to define a configuration struct and a utility function.
// In larger systems, such utility functions might be moved to a more general `utils` package
// if they are not strictly related to auth, or kept here if their primary use is within auth.
package auth

import (
	"fmt"
)

// Helper functions

// parseStringSlice converts an interface{} to a slice of strings.
// This is a general utility function.
func parseStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, len(raw))
	for i, val := range raw {
		result[i] = fmt.Sprint(val)
	}
	return result
}

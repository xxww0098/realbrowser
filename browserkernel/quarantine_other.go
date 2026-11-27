//go:build !darwin

package browserkernel

import "context"

func detectMacQuarantine(string) bool { return false }

// ClearMacQuarantine is a no-op on non-macOS platforms.
func ClearMacQuarantine(context.Context, string) error { return nil }

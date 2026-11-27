//go:build !darwin

package browserkernel

import (
	"context"
	"errors"
)

// unpackDMG is only supported on macOS.
func unpackDMG(context.Context, string, string) (string, error) {
	return "", errors.New("fpchromium: .dmg installation is only supported on macOS")
}

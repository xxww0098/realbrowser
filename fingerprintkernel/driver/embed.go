// Package driver provides the fingerprint profile reader and display settings for Node hosts.
package driver

import "embed"

// Files contains the Node module used by hosts that launch through a private driver.
//
//go:embed fingerprint-identity.mjs
var Files embed.FS

// Package downloader provides the optional Node transport for multi-source kernel downloads.
package downloader

import "embed"

// Files contains download-watch.mjs for hosts that bundle the Node driver.
//
//go:embed download-watch.mjs
var Files embed.FS

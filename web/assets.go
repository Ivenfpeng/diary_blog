// Package webassets embeds the public site and administration assets.
package webassets

import "embed"

// Assets is the immutable set of browser assets shipped with the application.
//
//go:embed templates static admin
var Assets embed.FS

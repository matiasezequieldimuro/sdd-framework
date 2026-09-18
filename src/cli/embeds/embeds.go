package embeds

import "embed"

// DefaultSDDResources embeds the default .sdd directory assets for initialization.
//
//go:embed all:default_sdd
var DefaultSDDResources embed.FS

// DefaultAdapterResources embeds agent-specific adapter assets for installation.
//
//go:embed all:default_adapters
var DefaultAdapterResources embed.FS

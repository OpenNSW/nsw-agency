// Package blobsource is a thin shim over github.com/OpenNSW/nsw/backend/pkg/blobsource.
// It re-exports the upstream types and factory so all consumers in this repo
// keep a single import path, and adds GetLayered — an nsw-agency-specific
// helper for layered primary+builtin lookups.
//
// Once the upstream module is published with a stable tag, every import of
// this package can be replaced with a direct import of the upstream one
// (keeping only GetLayered in an internal helper package).
package blobsource

import (
	"context"
	"fmt"

	upstream "github.com/OpenNSW/nsw/backend/pkg/blobsource"
)

// Type aliases — consumers see blobsource.Source / blobsource.Config etc.
// and never need to know which module owns the types.
type (
	Source = upstream.Source
	Config = upstream.Config
)

// NewFromConfig delegates to the upstream factory.
func NewFromConfig(ctx context.Context, cfg Config) (Source, error) {
	return upstream.NewFromConfig(ctx, cfg)
}

// ConfigTypeLabel returns a display label for a *Config, suitable for logging.
// A nil config (disabled source) returns "none".
func ConfigTypeLabel(cfg *Config) string {
	if cfg == nil {
		return "none"
	}
	return cfg.Type
}

// GetLayered fetches id by trying each source in order. nil sources are
// skipped, so callers can pass a nil layer to mean "disabled". The first
// source that returns ok=true wins, and its label (the corresponding entry in
// labels) is returned. A nil result with no error means no source had the ID.
//
// labels must be the same length as sources.
func GetLayered(ctx context.Context, id string, sources []Source, labels []string) ([]byte, string, error) {
	if len(sources) != len(labels) {
		return nil, "", fmt.Errorf("blobsource: sources/labels length mismatch (%d vs %d)", len(sources), len(labels))
	}
	for i, src := range sources {
		if src == nil {
			continue
		}
		data, ok, err := src.Get(ctx, id)
		if err != nil {
			return nil, labels[i], err
		}
		if ok {
			return data, labels[i], nil
		}
	}
	return nil, "", nil
}

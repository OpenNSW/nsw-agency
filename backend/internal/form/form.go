package form

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/OpenNSW/nsw-agency/backend/pkg/blobsource"
)

// FormsSubdir is the subdirectory under the built-in defaults root where form
// files live. Used by cmd/server to construct the built-in defaults source.
const FormsSubdir = "forms"

// FormStore resolves form definitions ({ schema, uiSchema }) by ID on demand.
// Forms are layered: primary source wins, built-in defaults are used on miss.
// Neither source is pre-fetched at construction time; the underlying sources
// handle caching (the GitHub source caches raw bytes after first fetch; the
// local source loads everything at init).
type FormStore struct {
	primary blobsource.Source
	builtin blobsource.Source
}

// NewFormStore builds a FormStore backed by the given sources. Either source
// may be nil to mean "not configured"; lookups skip nil layers. No blobs are
// fetched here — everything is resolved on first GetForm call.
func NewFormStore(_ context.Context, primary, builtin blobsource.Source) (*FormStore, error) {
	slog.Info("form store initialised",
		"primaryConfigured", primary != nil, "builtinConfigured", builtin != nil)
	return &FormStore{primary: primary, builtin: builtin}, nil
}

// GetForm fetches the raw JSON for the given form ID. It tries the primary
// source first and falls back to the built-in defaults on miss.
// Returns (nil, false, nil) when the ID is not found in either source.
func (fs *FormStore) GetForm(ctx context.Context, id string) (json.RawMessage, bool, error) {
	sources := []blobsource.Source{fs.primary, fs.builtin}
	labels := []string{"primary", "builtin"}

	data, layer, err := blobsource.GetLayered(ctx, id, sources, labels)
	if err != nil {
		return nil, false, fmt.Errorf("form %q: %w", id, err)
	}
	if data == nil {
		return nil, false, nil
	}
	if !json.Valid(data) {
		return nil, false, fmt.Errorf("form %q from %s source contains invalid JSON", id, layer)
	}
	return data, true, nil
}

// Close releases the underlying blob sources (stops GitHub background refresh
// goroutines). Safe to call multiple times.
func (fs *FormStore) Close() error {
	var firstErr error
	for _, src := range []blobsource.Source{fs.primary, fs.builtin} {
		if src != nil {
			if err := src.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

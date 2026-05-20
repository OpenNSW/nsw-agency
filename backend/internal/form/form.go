package form

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// FormsSubdir is the subdirectory under the config root where form files live.
const FormsSubdir = "forms"

// FormStore holds loaded form definitions ({ schema, uiSchema }) keyed by form ID.
// The form ID is the filename (without the .json extension).
type FormStore struct {
	forms map[string]json.RawMessage
}

// NewFormStore reads all .json files from <configDir>/forms into memory.
func NewFormStore(configDir string) (*FormStore, error) {
	dir := filepath.Join(configDir, FormsSubdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read forms directory %q: %w", dir, err)
	}

	forms := make(map[string]json.RawMessage)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read form file %q: %w", entry.Name(), err)
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("form file %q contains invalid JSON", entry.Name())
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		forms[id] = data
		slog.Info("loaded form", "id", id)
	}

	slog.Info("form store initialized", "count", len(forms))
	return &FormStore{forms: forms}, nil
}

// GetForm returns the raw JSON for the given form ID and whether it was found.
func (fs *FormStore) GetForm(id string) (json.RawMessage, bool) {
	form, ok := fs.forms[id]
	return form, ok
}

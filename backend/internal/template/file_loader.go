package template

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
)

// FileLoader implements Provider and Loader interfaces, retrieving template data from the local filesystem.
type FileLoader struct {
	taskConfigsDir  string
	formsDir        string
	taskConfigs     map[string]*taskconfig.TaskConfig
	forms           map[string]json.RawMessage
}

// NewFileLoader creates a new FileLoader pointing to the task configs and forms directories.
func NewFileLoader(taskConfigsDir string, formsDir string) *FileLoader {
	return &FileLoader{
		taskConfigsDir:  taskConfigsDir,
		formsDir:        formsDir,
		taskConfigs:     make(map[string]*taskconfig.TaskConfig),
		forms:           make(map[string]json.RawMessage),
	}
}

// Load reads task configurations first, collects all referenced form IDs,
// recursively scans formsDir, matches form JSON IDs against the collected set,
// and fails fast with an error if any referenced form IDs were not found.
func (l *FileLoader) Load() error {
	// 1. Load Task Configs recursively from taskConfigsDir
	referencedFormIDs := make(map[string]bool)
	err := filepath.WalkDir(l.taskConfigsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read task config file %q: %w", path, err)
		}

		var config taskconfig.TaskConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("task config file %q is invalid: %w", path, err)
		}

		id := strings.TrimSuffix(d.Name(), ".json")
		if config.TaskCode == "" {
			config.TaskCode = id
		}
		l.taskConfigs[config.TaskCode] = &config

		// Collect referenced form IDs
		if config.Forms.View != "" {
			referencedFormIDs[config.Forms.View] = true
		}
		if config.Forms.Review != "" {
			referencedFormIDs[config.Forms.Review] = true
		}

		slog.Info("loaded task config", "id", id, "taskCode", config.TaskCode)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to load task configs recursively: %w", err)
	}

	// 2. Scan formsDir recursively for matching jsonforms templates
	err = filepath.WalkDir(l.formsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read form file %q: %w", path, err)
		}

		// Quick parse to check if it has a top-level id matching what we collected
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("form file %q is invalid JSON: %w", path, err)
		}

		// Extract top-level "id" from JSON
		var id string
		if idJSON, ok := doc["id"]; ok {
			var parsedID string
			if err := json.Unmarshal(idJSON, &parsedID); err == nil && parsedID != "" {
				id = parsedID
			}
		}

		// Fallback to filename (without extension) if "id" is not in JSON
		if id == "" {
			id = strings.TrimSuffix(d.Name(), ".json")
		}

		// If this form ID is in the referenced set, load and cache it
		if referencedFormIDs[id] {
			l.forms[id] = data
			slog.Info("loaded matched form template", "id", id, "path", path)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to load form templates recursively: %w", err)
	}

	// 3. Error out if any referenced form IDs were not found
	for formID := range referencedFormIDs {
		if _, loaded := l.forms[formID]; !loaded {
			return fmt.Errorf("form %q referenced in task configs was not found in form templates", formID)
		}
	}

	slog.Info("template file loader initialized successfully", "forms_count", len(l.forms), "configs_count", len(l.taskConfigs))
	return nil
}

// GetTaskConfig retrieves the configuration for the given task code.
func (l *FileLoader) GetTaskConfig(taskCode string) (*taskconfig.TaskConfig, error) {
	if config, ok := l.taskConfigs[taskCode]; ok {
		return config, nil
	}
	return nil, fmt.Errorf("task config %q not found", taskCode)
}

// GetForm retrieves the raw JSON schema/uiSchema for the given form ID.
func (l *FileLoader) GetForm(formID string) (json.RawMessage, bool) {
	formBytes, ok := l.forms[formID]
	return formBytes, ok
}

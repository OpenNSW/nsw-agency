package internal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// TaskConfig is the per-taskCode configuration: UI metadata, references to
// forms in the form.FormStore, and outcome-to-status behavior.
type TaskConfig struct {
	TaskCode string        `json:"taskCode"`
	Meta     TaskMeta      `json:"meta"`
	Forms    TaskForms     `json:"forms"`
	Behavior *TaskBehavior `json:"behavior,omitempty"`
}

// TaskMeta contains UI metadata for the task.
type TaskMeta struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`
}

// TaskForms holds form IDs (filenames without .json) resolved against the form.FormStore.
type TaskForms struct {
	View   string `json:"view,omitempty"`
	Review string `json:"review,omitempty"`
}

// DefaultOutcomeField is the field name read from the review submission
// body when TaskBehavior.OutcomeField is not set.
const DefaultOutcomeField = "review_outcome"

// TaskBehavior defines automated logic based on task outcomes.
type TaskBehavior struct {
	// OutcomeField names the key in the review submission body whose value
	// is looked up in StatusMap. Defaults to "review_outcome" when empty.
	OutcomeField string            `json:"outcomeField,omitempty"`
	StatusMap    map[string]string `json:"statusMap,omitempty"`
}

// TaskConfigStore holds loaded task configurations keyed by task code.
type TaskConfigStore struct {
	configs         map[string]*TaskConfig
	defaultConfigID string
}

// TaskConfigsSubdir is the subdirectory under the config root where task config files live.
const TaskConfigsSubdir = "task-configs"

// NewTaskConfigStore reads all .json files from <configDir>/task-configs into memory.
// The task code is the filename without the .json extension.
func NewTaskConfigStore(configDir string, defaultConfigID string) (*TaskConfigStore, error) {
	dir := filepath.Join(configDir, TaskConfigsSubdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read task configs directory %q: %w", dir, err)
	}

	configs := make(map[string]*TaskConfig)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read task config file %q: %w", entry.Name(), err)
		}

		var config TaskConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("task config file %q is invalid: %w", entry.Name(), err)
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		if config.TaskCode == "" {
			config.TaskCode = id
		}
		configs[id] = &config
		slog.Info("loaded task config", "id", id)
	}

	slog.Info("task config store initialized", "count", len(configs), "defaultConfigID", defaultConfigID)
	return &TaskConfigStore{configs: configs, defaultConfigID: defaultConfigID}, nil
}

// GetConfig returns the configuration for the given task code, falling back to
// the configured default if one is set, or returning an error otherwise.
func (ts *TaskConfigStore) GetConfig(taskCode string) (*TaskConfig, error) {
	if config, ok := ts.configs[taskCode]; ok {
		return config, nil
	}
	if ts.defaultConfigID != "" {
		if def, ok := ts.configs[ts.defaultConfigID]; ok {
			return def, nil
		}
	}
	return nil, fmt.Errorf("task config %q not found", taskCode)
}

package taskconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/OpenNSW/nsw-agency/backend/pkg/blobsource"
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

// TaskConfigsSubdir is the subdirectory under the built-in defaults root where
// task config files live. Used by cmd/server to construct the built-in
// defaults source.
const TaskConfigsSubdir = "task-configs"

// TaskConfigStore resolves task configurations by task code on demand.
// Configs are layered: primary source wins, built-in defaults are used on miss.
// Raw bytes are cached by the underlying blobsource; no additional cache is
// kept here so that background manifest refreshes propagate immediately.
type TaskConfigStore struct {
	primary         blobsource.Source
	builtin         blobsource.Source
	defaultConfigID string
}

// NewTaskConfigStore builds a TaskConfigStore backed by the given sources.
// Either source may be nil to mean "not configured"; lookups skip nil layers.
// No configs are fetched at construction time; everything is resolved on first
// GetConfig call.
func NewTaskConfigStore(_ context.Context, primary, builtin blobsource.Source, defaultConfigID string) (*TaskConfigStore, error) {
	slog.Info("task config store initialised",
		"primaryConfigured", primary != nil,
		"builtinConfigured", builtin != nil,
		"defaultConfigID", defaultConfigID)
	return &TaskConfigStore{
		primary:         primary,
		builtin:         builtin,
		defaultConfigID: defaultConfigID,
	}, nil
}

// GetConfig fetches the configuration for the given task code, falling back to
// the configured default if one is set, or returning an error otherwise.
func (ts *TaskConfigStore) GetConfig(ctx context.Context, taskCode string) (*TaskConfig, error) {
	if cfg, err := ts.resolve(ctx, taskCode); err == nil {
		slog.Info("task config resolved", "taskCode", taskCode, "config", cfg)
		return cfg, nil
	}
	// Task code not found in either source; try the default.
	if ts.defaultConfigID != "" && ts.defaultConfigID != taskCode {
		if cfg, err := ts.resolve(ctx, ts.defaultConfigID); err == nil {
			slog.Info("task config resolved", "taskCode", ts.defaultConfigID, "config", cfg)
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("task config %q not found", taskCode)
}

// resolve fetches a config by its exact ID (no default fallback).
func (ts *TaskConfigStore) resolve(ctx context.Context, id string) (*TaskConfig, error) {
	sources := []blobsource.Source{ts.primary, ts.builtin}
	labels := []string{"primary", "builtin"}
	data, layer, err := blobsource.GetLayered(ctx, id, sources, labels)
	if err != nil {
		return nil, fmt.Errorf("task config %q: %w", id, err)
	}
	if data == nil {
		return nil, fmt.Errorf("task config %q not found", id)
	}

	var cfg TaskConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("task config %q from %s source is invalid: %w", id, layer, err)
	}
	if cfg.TaskCode == "" {
		cfg.TaskCode = id
	}
	slog.Info("loaded task config", "id", id, "layer", layer)
	return &cfg, nil
}

// Close releases the underlying blob sources (stops GitHub background refresh
// goroutines). Safe to call multiple times.
func (ts *TaskConfigStore) Close() error {
	var firstErr error
	for _, src := range []blobsource.Source{ts.primary, ts.builtin} {
		if src != nil {
			if err := src.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

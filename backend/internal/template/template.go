package template

import (
	"encoding/json"

	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
)

// Provider defines the interface to query loaded task configs and form schemas.
type Provider interface {
	// GetTaskConfig retrieves the configuration for a task code.
	GetTaskConfig(taskCode string) (*taskconfig.TaskConfig, error)

	// GetForm retrieves the raw JSON schema/uiSchema for a form ID.
	GetForm(formID string) (json.RawMessage, bool)

	// ListTaskConfigs returns all loaded task configurations.
	ListTaskConfigs() []*taskconfig.TaskConfig
}

// Loader defines the interface to load, parse, and validate templates.
type Loader interface {
	// Load processes all templates from the source and performs validation.
	Load() error
}

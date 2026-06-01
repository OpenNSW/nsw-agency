package taskconfig

// TaskConfig is the per-taskCode configuration: UI metadata, references to
// forms, and outcome-to-status behavior.
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

// TaskForms holds form IDs referenced by the task config.
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

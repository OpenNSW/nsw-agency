package taskconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// newTaskConfigsDir creates a temporary config root with an empty
// <root>/task-configs/ subdirectory and returns the root path.
func newTaskConfigsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, TaskConfigsSubdir), 0o755); err != nil {
		t.Fatalf("failed to create task-configs dir: %v", err)
	}
	return root
}

// writeTaskConfigFile writes content to <root>/task-configs/<name>.
func writeTaskConfigFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, TaskConfigsSubdir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestTaskConfigStore_LoadsValidConfigs(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "alpha.json", `{
		"taskCode": "alpha",
		"meta": {"title": "Alpha Review"},
		"forms": {"review": "alpha_review"}
	}`)
	writeTaskConfigFile(t, root, "beta.json", `{
		"meta": {"title": "Beta Review"},
		"forms": {"view": "beta_view", "review": "beta_review"},
		"behavior": {"statusMap": {"approve": "APPROVED"}}
	}`)

	store, err := NewTaskConfigStore(root, "")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	alpha, err := store.GetConfig("alpha")
	if err != nil {
		t.Fatalf("GetConfig(alpha) failed: %v", err)
	}
	if alpha.Meta.Title != "Alpha Review" {
		t.Errorf("expected alpha.Meta.Title = %q, got %q", "Alpha Review", alpha.Meta.Title)
	}
	if alpha.Forms.Review != "alpha_review" {
		t.Errorf("expected alpha.Forms.Review = %q, got %q", "alpha_review", alpha.Forms.Review)
	}

	beta, err := store.GetConfig("beta")
	if err != nil {
		t.Fatalf("GetConfig(beta) failed: %v", err)
	}
	// taskCode should be inferred from the filename when omitted.
	if beta.TaskCode != "beta" {
		t.Errorf("expected beta.TaskCode inferred from filename, got %q", beta.TaskCode)
	}
	if beta.Behavior == nil || beta.Behavior.StatusMap["approve"] != "APPROVED" {
		t.Errorf("expected beta.Behavior.StatusMap[approve] = APPROVED, got %v", beta.Behavior)
	}
}

func TestTaskConfigStore_SkipsNonJSONFiles(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "alpha.json", `{"meta":{"title":"A"}}`)
	writeTaskConfigFile(t, root, "readme.txt", `not a config`)
	writeTaskConfigFile(t, root, "config.yaml", `meta: { title: B }`)

	store, err := NewTaskConfigStore(root, "")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	if _, err := store.GetConfig("alpha"); err != nil {
		t.Errorf("expected alpha loaded, got error: %v", err)
	}
	if _, err := store.GetConfig("readme"); err == nil {
		t.Errorf("readme.txt should have been skipped")
	}
	if _, err := store.GetConfig("config"); err == nil {
		t.Errorf("config.yaml should have been skipped")
	}
}

func TestTaskConfigStore_DefaultFallback(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "default.json", `{"meta":{"title":"Generic Review"}}`)
	writeTaskConfigFile(t, root, "specific.json", `{"meta":{"title":"Specific Review"}}`)

	store, err := NewTaskConfigStore(root, "default")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	// Known taskCode returns its own config.
	specific, err := store.GetConfig("specific")
	if err != nil {
		t.Fatalf("GetConfig(specific) failed: %v", err)
	}
	if specific.Meta.Title != "Specific Review" {
		t.Errorf("expected specific.Meta.Title = %q, got %q", "Specific Review", specific.Meta.Title)
	}

	// Unknown taskCode falls back to default.
	got, err := store.GetConfig("unknown")
	if err != nil {
		t.Fatalf("expected default fallback for unknown taskCode, got error: %v", err)
	}
	if got.Meta.Title != "Generic Review" {
		t.Errorf("expected default fallback, got Meta.Title %q", got.Meta.Title)
	}
}

func TestTaskConfigStore_NoDefaultReturnsError(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "alpha.json", `{"meta":{"title":"Alpha"}}`)

	// defaultConfigID is empty: an unknown taskCode must return an error.
	store, err := NewTaskConfigStore(root, "")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	if _, err := store.GetConfig("missing"); err == nil {
		t.Errorf("expected error for missing taskCode when no default is set")
	}
}

func TestTaskConfigStore_DefaultIDNotPresent(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "alpha.json", `{"meta":{"title":"Alpha"}}`)

	// Configured default ID points at a file that doesn't exist.
	// Store should still construct (no constructor-level enforcement),
	// but lookups for unknown taskCodes must error since the default can't be resolved.
	store, err := NewTaskConfigStore(root, "nonexistent")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	if _, err := store.GetConfig("missing"); err == nil {
		t.Errorf("expected error when configured default ID is not in the store")
	}
}

func TestTaskConfigStore_ErrorOnInvalidJSON(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "broken.json", `{not valid`)

	_, err := NewTaskConfigStore(root, "")
	if err == nil {
		t.Fatalf("expected error when loading invalid JSON, got nil")
	}
}

func TestTaskConfigStore_ErrorOnMissingDir(t *testing.T) {
	root := t.TempDir()
	// Intentionally do not create root/task-configs.

	_, err := NewTaskConfigStore(root, "")
	if err == nil {
		t.Fatalf("expected error when task-configs directory is missing, got nil")
	}
}

func TestTaskConfigStore_OutcomeFieldUnmarshals(t *testing.T) {
	root := newTaskConfigsDir(t)
	writeTaskConfigFile(t, root, "labs.json", `{
		"meta": {"title": "Lab Results"},
		"behavior": {
			"outcomeField": "decision",
			"statusMap": {"pass": "APPROVED", "fail": "REJECTED"}
		}
	}`)

	store, err := NewTaskConfigStore(root, "")
	if err != nil {
		t.Fatalf("NewTaskConfigStore failed: %v", err)
	}

	cfg, err := store.GetConfig("labs")
	if err != nil {
		t.Fatalf("GetConfig(labs) failed: %v", err)
	}
	if cfg.Behavior == nil {
		t.Fatalf("expected Behavior to be set")
	}
	if cfg.Behavior.OutcomeField != "decision" {
		t.Errorf("expected OutcomeField = %q, got %q", "decision", cfg.Behavior.OutcomeField)
	}
}

func TestDefaultOutcomeFieldConstant(t *testing.T) {
	// Guard against accidental rename: the constant is part of the public
	// contract documented in task-configs.md and the .env.example.
	if DefaultOutcomeField != "review_outcome" {
		t.Errorf("DefaultOutcomeField changed: expected %q, got %q", "review_outcome", DefaultOutcomeField)
	}
}

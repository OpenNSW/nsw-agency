package template

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestConfigDir sets up a temporary directory with empty subdirectories for task-configs and forms.
func setupTestConfigDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "forms"), 0o755); err != nil {
		t.Fatalf("failed to create forms dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "task-configs"), 0o755); err != nil {
		t.Fatalf("failed to create task-configs dir: %v", err)
	}
	return root
}

func writeTestFile(t *testing.T, root, subDir, name, content string) {
	t.Helper()
	path := filepath.Join(root, subDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func TestFileLoader_LoadsValidTemplates(t *testing.T) {
	root := setupTestConfigDir(t)
	taskConfigsDir := filepath.Join(root, "task-configs")
	formsDir := filepath.Join(root, "forms")

	// Forms
	writeTestFile(t, root, "forms", "form1.json", `{"id":"custom-id-1","schema":{"type":"object"}}`)
	writeTestFile(t, root, "forms", "nested/form2.json", `{"uiSchema":{"type":"VerticalLayout"}}`)
	writeTestFile(t, root, "forms", "nested/workflow.json", `{"id":"workflow-id","task_type":"EXTERNAL_REVIEW"}`) // should be skipped (no schema/uiSchema)
	writeTestFile(t, root, "forms", "ignored.txt", `some non-json text`)
	
	// Unreferenced form (should NOT be loaded/cached)
	writeTestFile(t, root, "forms", "unreferenced_form.json", `{"id":"unreferenced-form-id","schema":{"type":"object"}}`)

	// Task configs
	writeTestFile(t, root, "task-configs", "task1.json", `{
		"taskCode": "task_code_1",
		"meta": {"title": "Task One"},
		"forms": {"view": "custom-id-1", "review": "form2"}
	}`)

	loader := NewFileLoader(taskConfigsDir, formsDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load failed unexpectedly: %v", err)
	}

	// Verify form1 is loaded under its custom ID
	if _, ok := loader.GetForm("custom-id-1"); !ok {
		t.Errorf("expected form with custom-id-1 to be loaded")
	}
	if _, ok := loader.GetForm("form1"); ok {
		t.Errorf("form1 should not be loaded by filename since it has a custom ID")
	}

	// Verify form2 is loaded under its filename (fallback)
	if _, ok := loader.GetForm("form2"); !ok {
		t.Errorf("expected form2 to be loaded under its filename")
	}

	// Verify non-form files were skipped
	if _, ok := loader.GetForm("workflow-id"); ok {
		t.Errorf("workflow.json should have been skipped as it is not a form")
	}
	if _, ok := loader.GetForm("ignored"); ok {
		t.Errorf("ignored.txt should have been skipped")
	}

	// Verify unreferenced form was skipped/not loaded
	if _, ok := loader.GetForm("unreferenced-form-id"); ok {
		t.Errorf("unreferenced form should not have been loaded into memory")
	}

	// Verify task config loaded
	config, err := loader.GetTaskConfig("task_code_1")
	if err != nil {
		t.Fatalf("failed to retrieve task config: %v", err)
	}
	if config.Meta.Title != "Task One" {
		t.Errorf("expected title 'Task One', got %q", config.Meta.Title)
	}
}

func TestFileLoader_ValidationFailsOnMissingForm(t *testing.T) {
	root := setupTestConfigDir(t)
	taskConfigsDir := filepath.Join(root, "task-configs")
	formsDir := filepath.Join(root, "forms")

	// Task config references a missing form
	writeTestFile(t, root, "task-configs", "task1.json", `{
		"taskCode": "task_code_1",
		"meta": {"title": "Task One"},
		"forms": {"view": "missing-form-id"}
	}`)

	loader := NewFileLoader(taskConfigsDir, formsDir)
	err := loader.Load()
	if err == nil {
		t.Fatalf("expected Load to fail due to missing form reference, but it succeeded")
	}

	expectedErr := `form "missing-form-id" referenced in task configs was not found`
	if err.Error() != expectedErr {
		t.Logf("got error: %v", err)
	}
}

func TestFileLoader_GetTaskConfigNotFound(t *testing.T) {
	root := setupTestConfigDir(t)
	taskConfigsDir := filepath.Join(root, "task-configs")
	formsDir := filepath.Join(root, "forms")

	loader := NewFileLoader(taskConfigsDir, formsDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Requesting an unknown config should return an error
	_, err := loader.GetTaskConfig("unknown_task")
	if err == nil {
		t.Fatalf("expected GetTaskConfig to return error for unknown task, but it succeeded")
	}
}

func TestFileLoader_MissingDir(t *testing.T) {
	root := t.TempDir()
	taskConfigsDir := filepath.Join(root, "task-configs")
	formsDir := filepath.Join(root, "forms")
	// forms and task-configs subdirs do not exist

	loader := NewFileLoader(taskConfigsDir, formsDir)
	if err := loader.Load(); err == nil {
		t.Fatalf("expected Load to fail when directories are missing, got nil")
	}
}

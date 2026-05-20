package form

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newFormsDir creates a temporary config root with an empty <root>/forms/
// subdirectory and returns the root path. Form files can be written into
// filepath.Join(root, FormsSubdir).
func newFormsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, FormsSubdir), 0o755); err != nil {
		t.Fatalf("failed to create forms dir: %v", err)
	}
	return root
}

// writeFormFile writes content to <root>/forms/<name>.
func writeFormFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, FormsSubdir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestFormStore_LoadsValidForms(t *testing.T) {
	root := newFormsDir(t)
	writeFormFile(t, root, "alpha.json", `{"schema":{"type":"object"},"uiSchema":{"type":"VerticalLayout"}}`)
	writeFormFile(t, root, "beta.json", `{"schema":{"type":"object","title":"Beta"}}`)

	store, err := NewFormStore(root)
	if err != nil {
		t.Fatalf("NewFormStore failed: %v", err)
	}

	if _, ok := store.GetForm("alpha"); !ok {
		t.Errorf("expected form alpha to be loaded")
	}
	if _, ok := store.GetForm("beta"); !ok {
		t.Errorf("expected form beta to be loaded")
	}
}

func TestFormStore_GetFormReturnsRawJSON(t *testing.T) {
	root := newFormsDir(t)
	body := `{"schema":{"type":"object","required":["foo"]},"uiSchema":{"type":"VerticalLayout"}}`
	writeFormFile(t, root, "alpha.json", body)

	store, err := NewFormStore(root)
	if err != nil {
		t.Fatalf("NewFormStore failed: %v", err)
	}

	raw, ok := store.GetForm("alpha")
	if !ok {
		t.Fatalf("expected alpha to be loaded")
	}

	// Verify the returned bytes round-trip through JSON unmarshal.
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("returned form is not valid JSON: %v", err)
	}
	if _, ok := got["schema"]; !ok {
		t.Errorf("expected schema field in returned form, got %v", got)
	}
}

func TestFormStore_SkipsNonJSONFiles(t *testing.T) {
	root := newFormsDir(t)
	writeFormFile(t, root, "alpha.json", `{"schema":{"type":"object"}}`)
	writeFormFile(t, root, "readme.txt", `this is not a form`)
	writeFormFile(t, root, "beta.yaml", `schema: {}`)

	store, err := NewFormStore(root)
	if err != nil {
		t.Fatalf("NewFormStore failed: %v", err)
	}

	if _, ok := store.GetForm("alpha"); !ok {
		t.Errorf("expected alpha to be loaded")
	}
	// IDs should be derived from .json filenames only, never from .txt/.yaml.
	if _, ok := store.GetForm("readme"); ok {
		t.Errorf("readme.txt should have been skipped")
	}
	if _, ok := store.GetForm("beta"); ok {
		t.Errorf("beta.yaml should have been skipped")
	}
}

func TestFormStore_GetFormMiss(t *testing.T) {
	root := newFormsDir(t)
	writeFormFile(t, root, "alpha.json", `{"schema":{"type":"object"}}`)

	store, err := NewFormStore(root)
	if err != nil {
		t.Fatalf("NewFormStore failed: %v", err)
	}

	if _, ok := store.GetForm("does-not-exist"); ok {
		t.Errorf("expected GetForm miss to return (_, false)")
	}
}

func TestFormStore_ErrorOnInvalidJSON(t *testing.T) {
	root := newFormsDir(t)
	writeFormFile(t, root, "broken.json", `{this is not valid json`)

	_, err := NewFormStore(root)
	if err == nil {
		t.Fatalf("expected error when loading invalid JSON, got nil")
	}
}

func TestFormStore_ErrorOnMissingDir(t *testing.T) {
	root := t.TempDir()
	// Intentionally do not create root/forms.

	_, err := NewFormStore(root)
	if err == nil {
		t.Fatalf("expected error when forms directory is missing, got nil")
	}
}

func TestFormStore_IgnoresSubdirectories(t *testing.T) {
	root := newFormsDir(t)
	// A nested directory under forms/ should be ignored, not recursed into.
	if err := os.MkdirAll(filepath.Join(root, FormsSubdir, "nested"), 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	writeFormFile(t, root, "nested/should_be_ignored.json", `{"schema":{}}`)
	writeFormFile(t, root, "top.json", `{"schema":{"type":"object"}}`)

	store, err := NewFormStore(root)
	if err != nil {
		t.Fatalf("NewFormStore failed: %v", err)
	}

	if _, ok := store.GetForm("top"); !ok {
		t.Errorf("expected top to be loaded")
	}
	if _, ok := store.GetForm("should_be_ignored"); ok {
		t.Errorf("nested file should not be discovered")
	}
	if _, ok := store.GetForm("nested/should_be_ignored"); ok {
		t.Errorf("nested file should not be discovered under any key")
	}
}

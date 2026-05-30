package form

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/pkg/blobsource"
)

func writeFormFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func newLocalSource(t *testing.T, dir string) blobsource.Source {
	t.Helper()
	src, err := blobsource.NewFromConfig(context.Background(), blobsource.Config{
		Type:     "local",
		LocalDir: dir,
	})
	if err != nil {
		t.Fatalf("NewFromConfig(local, %q): %v", dir, err)
	}
	return src
}

func newStore(t *testing.T, primary, builtin blobsource.Source) *FormStore {
	t.Helper()
	store, err := NewFormStore(context.Background(), primary, builtin)
	if err != nil {
		t.Fatalf("NewFormStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestFormStore_LoadsValidForms(t *testing.T) {
	dir := t.TempDir()
	writeFormFile(t, dir, "alpha.json", `{"schema":{"type":"object"},"uiSchema":{"type":"VerticalLayout"}}`)
	writeFormFile(t, dir, "beta.json", `{"schema":{"type":"object","title":"Beta"}}`)

	store := newStore(t, nil, newLocalSource(t, dir))

	ctx := context.Background()
	if _, ok, err := store.GetForm(ctx, "alpha"); err != nil || !ok {
		t.Errorf("expected form alpha to be loaded (ok=%v, err=%v)", ok, err)
	}
	if _, ok, err := store.GetForm(ctx, "beta"); err != nil || !ok {
		t.Errorf("expected form beta to be loaded (ok=%v, err=%v)", ok, err)
	}
}

func TestFormStore_GetFormReturnsRawJSON(t *testing.T) {
	dir := t.TempDir()
	body := `{"schema":{"type":"object","required":["foo"]},"uiSchema":{"type":"VerticalLayout"}}`
	writeFormFile(t, dir, "alpha.json", body)

	store := newStore(t, nil, newLocalSource(t, dir))

	raw, ok, err := store.GetForm(context.Background(), "alpha")
	if err != nil || !ok {
		t.Fatalf("expected alpha to be loaded (ok=%v, err=%v)", ok, err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("returned form is not valid JSON: %v", err)
	}
	if _, ok := got["schema"]; !ok {
		t.Errorf("expected schema field in returned form, got %v", got)
	}
}

func TestFormStore_GetFormMiss(t *testing.T) {
	dir := t.TempDir()
	writeFormFile(t, dir, "alpha.json", `{"schema":{"type":"object"}}`)

	store := newStore(t, nil, newLocalSource(t, dir))

	_, ok, err := store.GetForm(context.Background(), "does-not-exist")
	if ok || err != nil {
		t.Errorf("expected (nil, false, nil) for miss, got (ok=%v, err=%v)", ok, err)
	}
}

func TestFormStore_ErrorOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeFormFile(t, dir, "broken.json", `{this is not valid json`)

	store := newStore(t, nil, newLocalSource(t, dir))

	_, ok, err := store.GetForm(context.Background(), "broken")
	if ok || err == nil {
		t.Fatalf("expected error for invalid JSON, got ok=%v err=%v", ok, err)
	}
}

func TestFormStore_PrimaryWinsOnConflict(t *testing.T) {
	primaryDir := t.TempDir()
	builtinDir := t.TempDir()
	writeFormFile(t, primaryDir, "alpha.json", `{"schema":{"const":"primary"}}`)
	writeFormFile(t, builtinDir, "alpha.json", `{"schema":{"const":"builtin"}}`)

	store := newStore(t, newLocalSource(t, primaryDir), newLocalSource(t, builtinDir))

	raw, ok, err := store.GetForm(context.Background(), "alpha")
	if err != nil || !ok {
		t.Fatalf("expected alpha to load: ok=%v err=%v", ok, err)
	}
	if !containsStr(raw, `"const":"primary"`) {
		t.Errorf("expected primary content to win, got %s", raw)
	}
}

func TestFormStore_BuiltinFallback(t *testing.T) {
	primaryDir := t.TempDir()
	builtinDir := t.TempDir()
	writeFormFile(t, primaryDir, "primary_only.json", `{"schema":{"const":"p"}}`)
	writeFormFile(t, builtinDir, "builtin_only.json", `{"schema":{"const":"b"}}`)

	store := newStore(t, newLocalSource(t, primaryDir), newLocalSource(t, builtinDir))

	if _, ok, err := store.GetForm(context.Background(), "primary_only"); err != nil || !ok {
		t.Errorf("primary_only should resolve from primary")
	}
	if _, ok, err := store.GetForm(context.Background(), "builtin_only"); err != nil || !ok {
		t.Errorf("builtin_only should resolve from builtin fallback")
	}
}

func TestFormStore_NilPrimary(t *testing.T) {
	dir := t.TempDir()
	writeFormFile(t, dir, "alpha.json", `{"schema":{"type":"object"}}`)

	store := newStore(t, nil, newLocalSource(t, dir))

	if _, ok, err := store.GetForm(context.Background(), "alpha"); err != nil || !ok {
		t.Errorf("expected alpha to resolve from builtin when primary is disabled (nil)")
	}
}

func TestFormStore_ManifestPrimary(t *testing.T) {
	primaryDir := t.TempDir()
	nested := filepath.Join(primaryDir, "templates")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFormFile(t, nested, "user-form.json", `{"schema":{"type":"object","title":"UF"}}`)
	manifest := []byte(`{"byId":{"workflow-user-form":"templates/user-form.json"}}`)
	if err := os.WriteFile(filepath.Join(primaryDir, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	builtinDir := t.TempDir()
	writeFormFile(t, builtinDir, "default_review.json", `{"schema":{}}`)

	store := newStore(t, newLocalSource(t, primaryDir), newLocalSource(t, builtinDir))

	if _, ok, err := store.GetForm(context.Background(), "workflow-user-form"); err != nil || !ok {
		t.Errorf("expected manifest-resolved form to be loaded")
	}
	if _, ok, err := store.GetForm(context.Background(), "default_review"); err != nil || !ok {
		t.Errorf("expected builtin default to be loaded")
	}
}

func containsStr(haystack []byte, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == needle {
			return true
		}
	}
	return false
}

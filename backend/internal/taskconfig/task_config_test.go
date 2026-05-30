package taskconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/pkg/blobsource"
)

func writeTaskConfigFile(t *testing.T, dir, name, content string) {
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

func newStore(t *testing.T, primary, builtin blobsource.Source, defaultID string) *TaskConfigStore {
	t.Helper()
	store, err := NewTaskConfigStore(context.Background(), primary, builtin, defaultID)
	if err != nil {
		t.Fatalf("NewTaskConfigStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestTaskConfigStore_LoadsValidConfigs(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "alpha.json", `{
		"taskCode": "alpha",
		"meta": {"title": "Alpha Review"},
		"forms": {"review": "alpha_review"}
	}`)
	writeTaskConfigFile(t, dir, "beta.json", `{
		"meta": {"title": "Beta Review"},
		"forms": {"view": "beta_view", "review": "beta_review"},
		"behavior": {"statusMap": {"approve": "APPROVED"}}
	}`)

	store := newStore(t, nil, newLocalSource(t, dir), "")

	ctx := context.Background()
	alpha, err := store.GetConfig(ctx, "alpha")
	if err != nil {
		t.Fatalf("GetConfig(alpha) failed: %v", err)
	}
	if alpha.Meta.Title != "Alpha Review" {
		t.Errorf("expected Alpha Review, got %q", alpha.Meta.Title)
	}
	if alpha.Forms.Review != "alpha_review" {
		t.Errorf("expected alpha_review, got %q", alpha.Forms.Review)
	}

	beta, err := store.GetConfig(ctx, "beta")
	if err != nil {
		t.Fatalf("GetConfig(beta) failed: %v", err)
	}
	if beta.TaskCode != "beta" {
		t.Errorf("expected taskCode inferred from filename, got %q", beta.TaskCode)
	}
	if beta.Behavior == nil || beta.Behavior.StatusMap["approve"] != "APPROVED" {
		t.Errorf("expected APPROVED in statusMap, got %v", beta.Behavior)
	}
}

func TestTaskConfigStore_DefaultFallback(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "default.json", `{"meta":{"title":"Generic Review"}}`)
	writeTaskConfigFile(t, dir, "specific.json", `{"meta":{"title":"Specific Review"}}`)

	store := newStore(t, nil, newLocalSource(t, dir), "default")
	ctx := context.Background()

	specific, err := store.GetConfig(ctx, "specific")
	if err != nil {
		t.Fatalf("GetConfig(specific): %v", err)
	}
	if specific.Meta.Title != "Specific Review" {
		t.Errorf("got %q", specific.Meta.Title)
	}

	got, err := store.GetConfig(ctx, "unknown")
	if err != nil {
		t.Fatalf("expected default fallback for unknown, got error: %v", err)
	}
	if got.Meta.Title != "Generic Review" {
		t.Errorf("expected default fallback, got %q", got.Meta.Title)
	}
}

func TestTaskConfigStore_NoDefaultReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "alpha.json", `{"meta":{"title":"Alpha"}}`)

	store := newStore(t, nil, newLocalSource(t, dir), "")

	if _, err := store.GetConfig(context.Background(), "missing"); err == nil {
		t.Errorf("expected error for missing taskCode when no default is set")
	}
}

func TestTaskConfigStore_DefaultIDNotPresent(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "alpha.json", `{"meta":{"title":"Alpha"}}`)

	store := newStore(t, nil, newLocalSource(t, dir), "nonexistent")

	if _, err := store.GetConfig(context.Background(), "missing"); err == nil {
		t.Errorf("expected error when configured default ID is not in the store")
	}
}

func TestTaskConfigStore_ErrorOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "broken.json", `{not valid`)

	store := newStore(t, nil, newLocalSource(t, dir), "")

	_, err := store.GetConfig(context.Background(), "broken")
	if err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
}

func TestTaskConfigStore_PrimaryWinsOnConflict(t *testing.T) {
	primaryDir := t.TempDir()
	builtinDir := t.TempDir()
	writeTaskConfigFile(t, primaryDir, "shared.json", `{"meta":{"title":"Primary Override"}}`)
	writeTaskConfigFile(t, builtinDir, "shared.json", `{"meta":{"title":"Builtin Default"}}`)

	store := newStore(t, newLocalSource(t, primaryDir), newLocalSource(t, builtinDir), "")

	cfg, err := store.GetConfig(context.Background(), "shared")
	if err != nil {
		t.Fatalf("GetConfig(shared): %v", err)
	}
	if cfg.Meta.Title != "Primary Override" {
		t.Errorf("expected primary to win, got %q", cfg.Meta.Title)
	}
}

func TestTaskConfigStore_BuiltinFallback(t *testing.T) {
	primaryDir := t.TempDir()
	builtinDir := t.TempDir()
	writeTaskConfigFile(t, primaryDir, "primary_only.json", `{"meta":{"title":"P"}}`)
	writeTaskConfigFile(t, builtinDir, "default.json", `{"meta":{"title":"Default Review"}}`)

	store := newStore(t, newLocalSource(t, primaryDir), newLocalSource(t, builtinDir), "default")
	ctx := context.Background()

	if _, err := store.GetConfig(ctx, "primary_only"); err != nil {
		t.Errorf("primary_only should resolve, got %v", err)
	}
	def, err := store.GetConfig(ctx, "default")
	if err != nil {
		t.Fatalf("default should resolve from builtin, got %v", err)
	}
	if def.Meta.Title != "Default Review" {
		t.Errorf("expected builtin default, got %q", def.Meta.Title)
	}
	unknown, err := store.GetConfig(ctx, "unknown")
	if err != nil {
		t.Fatalf("unknown should fall back to default, got %v", err)
	}
	if unknown.Meta.Title != "Default Review" {
		t.Errorf("expected default fallback, got %q", unknown.Meta.Title)
	}
}

func TestTaskConfigStore_MultipleRetrievalsSucceed(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "alpha.json", `{"meta":{"title":"Alpha"}}`)

	store := newStore(t, nil, newLocalSource(t, dir), "")
	ctx := context.Background()

	for i := range 3 {
		cfg, err := store.GetConfig(ctx, "alpha")
		if err != nil {
			t.Fatalf("GetConfig call %d: %v", i+1, err)
		}
		if cfg.Meta.Title != "Alpha" {
			t.Errorf("call %d: expected Alpha, got %q", i+1, cfg.Meta.Title)
		}
	}
}

func TestTaskConfigStore_OutcomeFieldUnmarshals(t *testing.T) {
	dir := t.TempDir()
	writeTaskConfigFile(t, dir, "labs.json", `{
		"meta": {"title": "Lab Results"},
		"behavior": {
			"outcomeField": "decision",
			"statusMap": {"pass": "APPROVED", "fail": "REJECTED"}
		}
	}`)

	store := newStore(t, nil, newLocalSource(t, dir), "")

	cfg, err := store.GetConfig(context.Background(), "labs")
	if err != nil {
		t.Fatalf("GetConfig(labs): %v", err)
	}
	if cfg.Behavior == nil || cfg.Behavior.OutcomeField != "decision" {
		t.Errorf("expected OutcomeField=decision, got %v", cfg.Behavior)
	}
}

func TestDefaultOutcomeFieldConstant(t *testing.T) {
	if DefaultOutcomeField != "review_outcome" {
		t.Errorf("DefaultOutcomeField changed: expected %q, got %q", "review_outcome", DefaultOutcomeField)
	}
}

package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenNSW/nsw-agency/backend/internal/config"
	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/OpenNSW/nsw-agency/backend/internal/feedback"
)

// ---------- helpers ----------

// newTestStore creates an ApplicationStore for tests.
// When NSW_AGENCY_DB_DRIVER=postgres (set via env), it connects to the configured
// PostgreSQL instance and truncates the table before each test.
// Otherwise it falls back to an in-memory SQLite database.
func newTestStore(t *testing.T) *ApplicationStore {
	t.Helper()

	var cfg config.Config
	if os.Getenv("NSW_AGENCY_DB_DRIVER") == "postgres" {
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
	} else {
		cfg = config.Config{
			DB: database.Config{Driver: "sqlite", Path: ":memory:"},
		}
	}

	store, err := NewApplicationStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store (driver=%s): %v", cfg.DB.Driver, err)
	}

	// For persistent backends, clean tables before each test.
	if cfg.DB.Driver != "sqlite" || cfg.DB.Path != ":memory:" {
		if err := store.db.Exec("TRUNCATE TABLE applications").Error; err != nil {
			t.Fatalf("failed to truncate applications table: %v", err)
		}
		if err := store.db.Exec("TRUNCATE TABLE consignments CASCADE").Error; err != nil {
			t.Fatalf("failed to truncate consignments table: %v", err)
		}
	}

	return store
}

// seedRecord inserts a minimal ApplicationRecord and fails the test on error.
func seedRecord(t *testing.T, store *ApplicationStore, taskID string, data JSONB) {
	t.Helper()
	if data == nil {
		data = JSONB{"key": "value"}
	}
	err := store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        taskID,
		TaskCode:      "verification:123",
		ConsignmentID: "wf-seed",
		ServiceURL:    "http://test",
		Data:          data,
		Status:        "PENDING",
	})
	if err != nil {
		t.Fatalf("seedRecord(%s) failed: %v", taskID, err)
	}
}

// ---------- 1. Integration Testing: SQLite Connectivity ----------

func TestApplicationStore_SQLite_FileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_agency.db")

	_, err := NewApplicationStore(config.Config{DB: database.Config{Driver: "sqlite", Path: dbPath}})
	if err != nil {
		t.Fatalf("NewApplicationStore failed: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected .db file to be created at configured DBPath")
	}
}

func TestApplicationStore_SQLite_SchemaMigration(t *testing.T) {
	store := newTestStore(t)
	if !store.db.Migrator().HasTable(&ApplicationRecord{}) {
		t.Error("applications table was not created after migration")
	}
	if !store.db.Migrator().HasTable(&ConsignmentRecord{}) {
		t.Error("consignments table was not created after migration")
	}
}

// ---------- 2. Functional Testing: CRUD Operations ----------

func TestApplicationStore_CreateAndRetrieve(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-crud-1", JSONB{"key": "value"})

	fetched, err := store.GetByTaskID("task-crud-1")
	if err != nil {
		t.Fatalf("GetByTaskID failed: %v", err)
	}
	if fetched.TaskID != "task-crud-1" {
		t.Errorf("expected TaskID 'task-crud-1', got %q", fetched.TaskID)
	}
	if fetched.Status != "PENDING" {
		t.Errorf("expected Status 'PENDING', got %q", fetched.Status)
	}
	if fetched.Data["key"] != "value" {
		t.Errorf("expected Data['key'] = 'value', got %v", fetched.Data["key"])
	}
}

func TestApplicationStore_GetByTaskID_NotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetByTaskID("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent task ID")
	}
}

func TestApplicationStore_UpdateStatus(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-status-1", nil)

	if err := store.UpdateStatus("task-status-1", "APPROVED", map[string]any{"reason": "ok"}); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	app, _ := store.GetByTaskID("task-status-1")
	if app.Status != "APPROVED" {
		t.Errorf("expected Status 'APPROVED', got %q", app.Status)
	}
	if app.ReviewedAt == nil {
		t.Error("expected ReviewedAt to be set after status update")
	}
}

func TestApplicationStore_UpdateStatus_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.UpdateStatus("nonexistent", "APPROVED", map[string]any{})
	if err == nil {
		t.Error("expected error when updating non-existent task")
	}
}

func TestApplicationStore_Delete(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-delete-1", nil)

	if err := store.Delete("task-delete-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := store.GetByTaskID("task-delete-1")
	if err == nil {
		t.Error("expected error after deleting task")
	}
}

// ---------- 3. Functional Testing: JSONB Serialization ----------

func TestApplicationStore_JSONB_DeepNesting(t *testing.T) {
	store := newTestStore(t)

	deepData := JSONB{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": "deep_value",
				"array":  []any{"a", "b", "c"},
			},
		},
		"boolean": true,
		"number":  42.5,
	}

	seedRecord(t, store, "task-jsonb-1", deepData)

	fetched, err := store.GetByTaskID("task-jsonb-1")
	if err != nil {
		t.Fatalf("GetByTaskID failed: %v", err)
	}

	// Verify deep nesting round-trip
	level1, ok := fetched.Data["level1"].(map[string]any)
	if !ok {
		t.Fatalf("expected level1 to be map, got %T", fetched.Data["level1"])
	}
	level2, ok := level1["level2"].(map[string]any)
	if !ok {
		t.Fatalf("expected level2 to be map, got %T", level1["level2"])
	}
	if level2["level3"] != "deep_value" {
		t.Errorf("expected level3 = 'deep_value', got %v", level2["level3"])
	}

	// Verify array round-trip
	arr, ok := level2["array"].([]any)
	if !ok {
		t.Fatalf("expected array to be []any, got %T", level2["array"])
	}
	if len(arr) != 3 || arr[0] != "a" {
		t.Errorf("expected array [a,b,c], got %v", arr)
	}

	// Verify numeric round-trip (JSON numbers are float64 in Go)
	if fetched.Data["number"] != 42.5 {
		t.Errorf("expected number = 42.5, got %v", fetched.Data["number"])
	}

	// Verify boolean round-trip
	if fetched.Data["boolean"] != true {
		t.Errorf("expected boolean = true, got %v", fetched.Data["boolean"])
	}
}

func TestApplicationStore_JSONB_NilData(t *testing.T) {
	store := newTestStore(t)

	err := store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        "task-nil-data",
		TaskCode:      "verification:123",
		ConsignmentID: "wf-1",
		ServiceURL:    "http://test",
		Data:          nil,
	})
	if err != nil {
		t.Fatalf("CreateOrUpdate with nil JSONB failed: %v", err)
	}

	fetched, _ := store.GetByTaskID("task-nil-data")
	if fetched.Data != nil {
		t.Errorf("expected nil Data, got %v", fetched.Data)
	}
}

// ---------- 4. Functional Testing: Pagination ----------

func TestApplicationStore_List_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Seed 5 records with different statuses
	for i := 0; i < 3; i++ {
		seedRecord(t, store, fmt.Sprintf("task-pend-%d", i), nil)
	}
	for i := 0; i < 2; i++ {
		taskID := fmt.Sprintf("task-approved-%d", i)
		seedRecord(t, store, taskID, nil)
		_ = store.UpdateStatus(taskID, "APPROVED", map[string]any{})
	}

	// List all
	apps, total, err := store.List(ctx, "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(apps) != 5 {
		t.Errorf("expected 5 apps, got %d", len(apps))
	}

	// List with status filter
	_, total, err = store.List(ctx, "APPROVED", "", "", 0, 10)
	if err != nil {
		t.Fatalf("List with status filter failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 approved, got %d", total)
	}

	// List with pagination
	apps, _, err = store.List(ctx, "", "", "", 0, 2)
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps with limit=2, got %d", len(apps))
	}

	// List with offset
	apps, _, err = store.List(ctx, "", "", "", 3, 10)
	if err != nil {
		t.Fatalf("List with offset failed: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps with offset=3, got %d", len(apps))
	}
}

func TestApplicationStore_List_ConsignmentFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seedRecord(t, store, "t1", nil) // consignment: wf-seed (default from seedRecord)
	seedRecord(t, store, "t2", nil)

	// Create another consignment
	err := store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        "t3",
		ConsignmentID: "wf-custom",
		Status:        "PENDING",
	})
	if err != nil {
		t.Fatalf("failed to seed wf-custom: %v", err)
	}

	// Filter by wf-seed
	apps, total, err := store.List(ctx, "", "wf-seed", "", 0, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 apps for wf-seed, got %d", total)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps returned, got %d", len(apps))
	}

	// Filter by wf-custom
	_, total, err = store.List(ctx, "", "wf-custom", "", 0, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 app for wf-custom, got %d", total)
	}
}

func TestApplicationStore_ListConsignments(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Seed records across 3 consignments
	// WF1: 2 tasks
	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "wf1-t1", TaskCode: "test", ConsignmentID: "wf1", ServiceURL: "http://test", Status: "PENDING"})
	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "wf1-t2", TaskCode: "test", ConsignmentID: "wf1", ServiceURL: "http://test", Status: "APPROVED"})

	// WF2: 1 task
	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "wf2-t1", TaskCode: "test", ConsignmentID: "wf2", ServiceURL: "http://test", Status: "PENDING"})

	// WF3: 1 task
	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "wf3-t1", TaskCode: "test", ConsignmentID: "wf3", ServiceURL: "http://test", Status: "REJECTED"})

	// List consignments
	summaries, total, err := store.ListConsignments(ctx, "", 0, 10)
	if err != nil {
		t.Fatalf("ListConsignments failed: %v", err)
	}

	if total != 3 {
		t.Errorf("expected 3 unique consignments, got %d", total)
	}
	if len(summaries) != 3 {
		t.Errorf("expected 3 summaries returned, got %d", len(summaries))
	}

	// Verify task counts
	foundWF1 := false
	for _, s := range summaries {
		if s.ConsignmentID == "wf1" {
			foundWF1 = true
			if s.TaskCount != 2 {
				t.Errorf("expected 2 tasks for wf1, got %d", s.TaskCount)
			}
		}
	}
	if !foundWF1 {
		t.Error("wf1 not found in summaries")
	}
}

func TestApplicationStore_ListConsignments_Search(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "t1", ConsignmentID: "alpha-wf", Status: "PENDING"})
	_ = store.CreateOrUpdate(&ApplicationRecord{TaskID: "t2", ConsignmentID: "beta-wf", Status: "PENDING"})

	summaries, total, err := store.ListConsignments(ctx, "alpha", 0, 10)
	if err != nil {
		t.Fatalf("ListConsignments failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if summaries[0].ConsignmentID != "alpha-wf" {
		t.Errorf("expected alpha-wf, got %s", summaries[0].ConsignmentID)
	}
}

// ---------- 5. Functional Testing: Feedback & Transactions ----------

func TestApplicationStore_AppendFeedback(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-fb-1", nil)

	feedback1 := feedback.Entry{Content: map[string]any{"comment": "needs revision"}, Round: 1}
	if err := store.AppendFeedback("task-fb-1", feedback1); err != nil {
		t.Fatalf("AppendFeedback round 1 failed: %v", err)
	}

	app, _ := store.GetByTaskID("task-fb-1")
	if app.Status != "FEEDBACK_REQUESTED" {
		t.Errorf("expected FEEDBACK_REQUESTED after feedback, got %q", app.Status)
	}
	if len(app.AgencyFeedbackHistory) != 1 {
		t.Errorf("expected 1 feedback entry, got %d", len(app.AgencyFeedbackHistory))
	}

	// Append a second round
	feedback2 := feedback.Entry{Content: map[string]any{"comment": "still needs work"}, Round: 2}
	if err := store.AppendFeedback("task-fb-1", feedback2); err != nil {
		t.Fatalf("AppendFeedback round 2 failed: %v", err)
	}

	app, _ = store.GetByTaskID("task-fb-1")
	if len(app.AgencyFeedbackHistory) != 2 {
		t.Errorf("expected 2 feedback entries, got %d", len(app.AgencyFeedbackHistory))
	}
	if app.AgencyFeedbackHistory[1].Content["comment"] != "still needs work" {
		t.Errorf("unexpected second feedback comment: %v", app.AgencyFeedbackHistory[1])
	}
}

func TestApplicationStore_AppendFeedback_NonExistent(t *testing.T) {
	store := newTestStore(t)

	err := store.AppendFeedback("nonexistent", feedback.Entry{Content: map[string]any{"comment": "nope"}})
	if err == nil {
		t.Error("expected error for feedback on non-existent task")
	}
}

// ---------- 6. Functional Testing: Resubmission Flow ----------

func TestApplicationStore_UpdateDataAndResetStatus(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-resub-1", JSONB{"old": "data"})

	// Simulate NSW Agency requesting feedback
	_ = store.AppendFeedback("task-resub-1", feedback.Entry{Content: map[string]any{"comment": "fix it"}})

	app, _ := store.GetByTaskID("task-resub-1")
	if app.Status != "FEEDBACK_REQUESTED" {
		t.Fatalf("expected FEEDBACK_REQUESTED, got %q", app.Status)
	}

	// Simulate trader resubmission
	newData := map[string]any{"new": "data", "updated": true}
	if err := store.UpdateDataAndResetStatus("task-resub-1", newData); err != nil {
		t.Fatalf("UpdateDataAndResetStatus failed: %v", err)
	}

	app, _ = store.GetByTaskID("task-resub-1")
	if app.Status != "PENDING" {
		t.Errorf("expected PENDING after resubmission, got %q", app.Status)
	}
	if app.Data["new"] != "data" {
		t.Errorf("expected updated data, got %v", app.Data)
	}
}

// ---------- 7. Functional Testing: Consignment Table ----------

func TestApplicationStore_ConsignmentUpsert(t *testing.T) {
	store := newTestStore(t)

	// Two CreateOrUpdate calls with the same consignment_id should result in one consignment row.
	if err := store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        "dup-t1",
		TaskCode:      "test",
		ConsignmentID: "dup-wf",
		ServiceURL:    "http://test",
		Status:        "PENDING",
	}); err != nil {
		t.Fatalf("first CreateOrUpdate failed: %v", err)
	}
	if err := store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        "dup-t2",
		TaskCode:      "test",
		ConsignmentID: "dup-wf",
		ServiceURL:    "http://test",
		Status:        "PENDING",
	}); err != nil {
		t.Fatalf("second CreateOrUpdate failed: %v", err)
	}

	var count int64
	if err := store.db.Model(&ConsignmentRecord{}).Where("id = ?", "dup-wf").Count(&count).Error; err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 consignment row for dup-wf, got %d", count)
	}
}

func TestApplicationStore_UpdateStatus_PropagatesConsignment(t *testing.T) {
	store := newTestStore(t)
	seedRecord(t, store, "task-prop-1", nil)

	if err := store.UpdateStatus("task-prop-1", "APPROVED", map[string]any{}); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	var cr ConsignmentRecord
	if err := store.db.First(&cr, "id = ?", "wf-seed").Error; err != nil {
		t.Fatalf("failed to fetch consignment: %v", err)
	}
	if cr.Status != "APPROVED" {
		t.Errorf("expected consignment status 'APPROVED', got %q", cr.Status)
	}
}

func TestApplicationStore_Backfill(t *testing.T) {
	store := newTestStore(t)

	// PostgreSQL enforces the FK constraint, so direct inserts without a consignment
	// row are rejected. In production this scenario (applications with no consignments)
	// only occurs before the FK is applied — i.e., exactly the moment backfill runs in
	// NewApplicationStore (between the two AutoMigrate calls). Skip on postgres.
	if store.db.Name() == "postgres" {
		t.Skip("backfill pre-condition (FK-free inserts) cannot be simulated on PostgreSQL")
	}

	// Insert application rows directly, bypassing CreateOrUpdate (simulates pre-migration data).
	apps := []ApplicationRecord{
		{TaskID: "bf-t1", TaskCode: "test", ConsignmentID: "bf-wf1", ServiceURL: "http://test", Status: "PENDING"},
		{TaskID: "bf-t2", TaskCode: "test", ConsignmentID: "bf-wf1", ServiceURL: "http://test", Status: "APPROVED"},
		{TaskID: "bf-t3", TaskCode: "test", ConsignmentID: "bf-wf2", ServiceURL: "http://test", Status: "PENDING"},
	}
	for _, a := range apps {
		if err := store.db.Create(&a).Error; err != nil {
			t.Fatalf("direct insert failed: %v", err)
		}
	}

	// Delete consignment rows to simulate a pre-migration state.
	if err := store.db.Exec("DELETE FROM consignments").Error; err != nil {
		t.Fatalf("failed to clear consignments: %v", err)
	}

	if err := backfillConsignments(store.db); err != nil {
		t.Fatalf("backfillConsignments failed: %v", err)
	}

	var count int64
	if err := store.db.Model(&ConsignmentRecord{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 consignment rows after backfill, got %d", count)
	}
}

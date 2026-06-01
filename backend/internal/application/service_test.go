package application

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

<<<<<<< HEAD
=======
	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
>>>>>>> 578c96b (update configs to lookup jsonforms by id, and add ability to point to nsw-srilanka or one-trade-tempalte repos to get the jsonforms template, to avoid duplication)
	"github.com/OpenNSW/nsw-agency/backend/internal/template"
	"github.com/OpenNSW/nsw-agency/backend/pkg/httpclient"
)

// writeTaskConfigFile writes content to <root>/task-configs/<name>.
func writeTaskConfigFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, "task-configs", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// writeFormFile writes content to <root>/forms/<name>.
func writeFormFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, "forms", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// ---------- service test harness ----------

// callbackCapture records the body of POSTs made to the test callback server.
type callbackCapture struct {
	mu    sync.Mutex
	calls [][]byte
}

func (c *callbackCapture) record(body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, body)
}

func (c *callbackCapture) lastCall() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		return nil
	}
	var got map[string]any
	_ = json.Unmarshal(c.calls[len(c.calls)-1], &got)
	return got
}

// newCallbackServer returns an httptest server that responds 200 OK to any POST
// and captures the request body for assertions.
func newCallbackServer(t *testing.T) (*httptest.Server, *callbackCapture) {
	t.Helper()
	capture := &callbackCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capture.record(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, capture
}

// serviceHarness wires the in-memory dependencies required to exercise
// Service end-to-end against a stub callback server.
type serviceHarness struct {
	t                *testing.T
	store            *ApplicationStore
	templateProvider template.Provider
	httpClient       *httpclient.Client
	callbackURL      string
	capture          *callbackCapture
	service          Service
}

// newServiceHarness constructs the harness with config and form files placed
// under writeFn before the stores are initialized.
//
// writeFn receives the config root path and is expected to populate
// <root>/task-configs/ and <root>/forms/ as needed.
func newServiceHarness(t *testing.T, writeFn func(root string)) *serviceHarness {
	t.Helper()

	root := t.TempDir()
	for _, sub := range []string{"task-configs", "forms"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatalf("failed to create %s dir: %v", sub, err)
		}
	}
	if writeFn != nil {
		writeFn(root)
	}

	store := newTestStore(t)

<<<<<<< HEAD
	loader := template.NewFileLoader(filepath.Join(root, "task-configs"), filepath.Join(root, "forms"))
=======
	loader := template.NewFileLoader(root, defaultConfigID)
>>>>>>> 578c96b (update configs to lookup jsonforms by id, and add ability to point to nsw-srilanka or one-trade-tempalte repos to get the jsonforms template, to avoid duplication)
	if err := loader.Load(); err != nil {
		t.Fatalf("FileLoader.Load failed: %v", err)
	}

	srv, capture := newCallbackServer(t)
	hc := httpclient.NewClientBuilder().Build()

	svc := NewService(store, loader, hc)
	t.Cleanup(func() { _ = svc.Close() })

	return &serviceHarness{
		t:                t,
		store:            store,
		templateProvider: loader,
		httpClient:       hc,
		callbackURL:      srv.URL,
		capture:          capture,
		service:          svc,
	}
}

// seed inserts an application record with the harness's callback URL as ServiceURL.
func (h *serviceHarness) seed(taskID, taskCode string, data JSONB) {
	h.t.Helper()
	if data == nil {
		data = JSONB{"field": "value"}
	}
	err := h.store.CreateOrUpdate(&ApplicationRecord{
		TaskID:        taskID,
		TaskCode:      taskCode,
		ConsignmentID: "wf-test",
		ServiceURL:    h.callbackURL,
		Data:          data,
		Status:        "PENDING",
	})
	if err != nil {
		h.t.Fatalf("failed to seed record: %v", err)
	}
}

// statusOf reads the latest status of the record from the database.
func (h *serviceHarness) statusOf(taskID string) string {
	h.t.Helper()
	rec, err := h.store.GetByTaskID(taskID)
	if err != nil {
		h.t.Fatalf("failed to load record: %v", err)
	}
	return rec.Status
}

// ---------- ReviewApplication: status derivation ----------

func TestReviewApplication_StatusFromStatusMap(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		writeTaskConfigFile(t, root, "alpha.json", `{
			"meta": {"title": "Alpha"},
			"behavior": {
				"statusMap": {
					"approve": "APPROVED",
					"reject":  "REJECTED",
					"needs_more_info": "FEEDBACK_REQUESTED"
				}
			}
		}`)
	})
	h.seed("t-approve", "alpha", nil)
	h.seed("t-reject", "alpha", nil)
	h.seed("t-feedback", "alpha", nil)

	cases := []struct {
		taskID  string
		outcome string
		want    string
	}{
		{"t-approve", "approve", "APPROVED"},
		{"t-reject", "reject", "REJECTED"},
		{"t-feedback", "needs_more_info", "FEEDBACK_REQUESTED"},
	}
	for _, tc := range cases {
		t.Run(tc.outcome, func(t *testing.T) {
			err := h.service.ReviewApplication(context.Background(), tc.taskID, map[string]any{
				"review_outcome": tc.outcome,
			})
			if err != nil {
				t.Fatalf("ReviewApplication failed: %v", err)
			}
			if got := h.statusOf(tc.taskID); got != tc.want {
				t.Errorf("status: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReviewApplication_DefaultsToDONE_OutcomeNotInMap(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		writeTaskConfigFile(t, root, "alpha.json", `{
			"meta": {"title": "Alpha"},
			"behavior": {"statusMap": {"approve": "APPROVED"}}
		}`)
	})
	h.seed("t-unknown", "alpha", nil)

	err := h.service.ReviewApplication(context.Background(), "t-unknown", map[string]any{
		"review_outcome": "totally_made_up",
	})
	if err != nil {
		t.Fatalf("ReviewApplication failed: %v", err)
	}
	if got := h.statusOf("t-unknown"); got != "DONE" {
		t.Errorf("status: got %q, want DONE (unmapped outcome should fall through)", got)
	}
}

func TestReviewApplication_DefaultsToDONE_NoStatusMap(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		// Config exists but defines no behavior/statusMap.
		writeTaskConfigFile(t, root, "alpha.json", `{"meta": {"title": "Alpha"}}`)
	})
	h.seed("t-no-map", "alpha", nil)

	err := h.service.ReviewApplication(context.Background(), "t-no-map", map[string]any{
		"review_outcome": "approve",
	})
	if err != nil {
		t.Fatalf("ReviewApplication failed: %v", err)
	}
	if got := h.statusOf("t-no-map"); got != "DONE" {
		t.Errorf("status: got %q, want DONE", got)
	}
}

func TestReviewApplication_DefaultsToDONE_NoConfig(t *testing.T) {
	h := newServiceHarness(t, nil)
	h.seed("t-no-config", "no-such-task", nil)

	err := h.service.ReviewApplication(context.Background(), "t-no-config", map[string]any{
		"review_outcome": "approve",
	})
	if err != nil {
		t.Fatalf("ReviewApplication failed: %v", err)
	}
	if got := h.statusOf("t-no-config"); got != "DONE" {
		t.Errorf("status: got %q, want DONE", got)
	}
}

// ---------- ReviewApplication: outcomeField override ----------

func TestReviewApplication_OutcomeFieldOverride(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		writeTaskConfigFile(t, root, "labs.json", `{
			"meta": {"title": "Lab Results"},
			"behavior": {
				"outcomeField": "decision",
				"statusMap": {"pass": "APPROVED", "fail": "REJECTED"}
			}
		}`)
	})

	t.Run("custom field hit", func(t *testing.T) {
		h.seed("t-pass", "labs", nil)
		err := h.service.ReviewApplication(context.Background(), "t-pass", map[string]any{
			"decision": "pass",
		})
		if err != nil {
			t.Fatalf("ReviewApplication failed: %v", err)
		}
		if got := h.statusOf("t-pass"); got != "APPROVED" {
			t.Errorf("status: got %q, want APPROVED (decision=pass)", got)
		}
	})

	t.Run("default field ignored when override set", func(t *testing.T) {
		h.seed("t-defaultignored", "labs", nil)
		// review_outcome is the default name but the config asked for "decision",
		// so the default name should NOT be honored.
		err := h.service.ReviewApplication(context.Background(), "t-defaultignored", map[string]any{
			"review_outcome": "pass",
		})
		if err != nil {
			t.Fatalf("ReviewApplication failed: %v", err)
		}
		if got := h.statusOf("t-defaultignored"); got != "DONE" {
			t.Errorf("status: got %q, want DONE (review_outcome should be ignored when outcomeField=decision)", got)
		}
	})
}

// ---------- ReviewApplication: callback dispatch ----------

func TestReviewApplication_CallsServiceURL(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		writeTaskConfigFile(t, root, "alpha.json", `{
			"meta": {"title": "Alpha"},
			"behavior": {"statusMap": {"approve": "APPROVED"}}
		}`)
	})
	h.seed("t-callback", "alpha", nil)

	err := h.service.ReviewApplication(context.Background(), "t-callback", map[string]any{
		"review_outcome": "approve",
		"comment":        "lgtm",
	})
	if err != nil {
		t.Fatalf("ReviewApplication failed: %v", err)
	}

	body := h.capture.lastCall()
	if body == nil {
		t.Fatalf("expected callback to be invoked, got no calls")
	}
	if body["task_id"] != "t-callback" {
		t.Errorf("callback task_id: got %v, want t-callback", body["task_id"])
	}
	payload, ok := body["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload object in callback, got %T", body["payload"])
	}
	if payload["action"] != "AGENCY_VERIFICATION" {
		t.Errorf("callback payload.action: got %v, want AGENCY_VERIFICATION", payload["action"])
	}
	content, ok := payload["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload.content object, got %T", payload["content"])
	}
	if content["review_outcome"] != "approve" || content["comment"] != "lgtm" {
		t.Errorf("payload.content forwarded incorrectly: got %v", content)
	}
}

// ---------- GetApplication: form resolution ----------

func TestGetApplication_ResolvesFormReferences(t *testing.T) {
	h := newServiceHarness(t, func(root string) {
		writeTaskConfigFile(t, root, "alpha.json", `{
			"meta": {"title": "Alpha", "category": "Test", "description": "Test task", "icon": "emoji:📋"},
			"forms": {"view": "alpha_view", "review": "alpha_review"}
		}`)
		writeFormFile(t, root, "alpha_view.json", `{"schema":{"type":"object","title":"View"},"uiSchema":{"type":"VerticalLayout"}}`)
		writeFormFile(t, root, "alpha_review.json", `{"schema":{"type":"object","title":"Review"},"uiSchema":{"type":"VerticalLayout"}}`)
	})
	h.seed("t-1", "alpha", JSONB{"submittedField": "submittedValue"})

	app, err := h.service.GetApplication(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
	}

	if app.Title != "Alpha" {
		t.Errorf("Title: got %q, want %q", app.Title, "Alpha")
	}
	if app.Description != "Test task" {
		t.Errorf("Description: got %q, want %q", app.Description, "Test task")
	}
	if app.Icon != "emoji:📋" {
		t.Errorf("Icon: got %q, want %q", app.Icon, "emoji:📋")
	}
	if app.Category != "Test" {
		t.Errorf("Category: got %q, want %q", app.Category, "Test")
	}

	if app.DataForm == nil {
		t.Errorf("expected DataForm to be attached")
	} else {
		var view map[string]any
		if err := json.Unmarshal(app.DataForm, &view); err != nil {
			t.Errorf("DataForm not valid JSON: %v", err)
		}
		if schema, ok := view["schema"].(map[string]any); !ok || schema["title"] != "View" {
			t.Errorf("DataForm content unexpected: %v", view)
		}
	}
	if app.AgencyForm == nil {
		t.Errorf("expected AgencyForm to be attached")
	} else {
		var review map[string]any
		if err := json.Unmarshal(app.AgencyForm, &review); err != nil {
			t.Errorf("AgencyForm not valid JSON: %v", err)
		}
		if schema, ok := review["schema"].(map[string]any); !ok || schema["title"] != "Review" {
			t.Errorf("AgencyForm content unexpected: %v", review)
		}
	}
}

func TestGetApplication_MissingFormRef_FailsLoader(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"task-configs", "forms"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatalf("failed to create %s dir: %v", sub, err)
		}
	}
	writeTaskConfigFile(t, root, "alpha.json", `{
		"meta": {"title": "Alpha"},
		"forms": {"view": "missing_view", "review": "missing_review"}
	}`)

	loader := template.NewFileLoader(root, "")
	if err := loader.Load(); err == nil {
		t.Errorf("expected Loader.Load to fail due to missing form references, but got nil")
	}
}

func TestGetApplication_NoConfig_OmitsMetadata(t *testing.T) {
	h := newServiceHarness(t, nil)
	h.seed("t-orphan", "no-config-for-this", nil)

	app, err := h.service.GetApplication(context.Background(), "t-orphan")
	if err != nil {
		t.Fatalf("GetApplication failed: %v", err)
	}
	if app.Title != "" || app.Category != "" || app.Icon != "" || app.Description != "" {
		t.Errorf("expected empty metadata when no config found, got title=%q desc=%q icon=%q cat=%q",
			app.Title, app.Description, app.Icon, app.Category)
	}
	if app.DataForm != nil || app.AgencyForm != nil {
		t.Errorf("expected nil forms when no config found")
	}
}

func TestGetApplication_NotFound(t *testing.T) {
	h := newServiceHarness(t, nil)
	_, err := h.service.GetApplication(context.Background(), "does-not-exist")
	if err != ErrApplicationNotFound {
		t.Errorf("expected ErrApplicationNotFound, got %v", err)
	}
}

# Task Configurations

A **task config** is the per-`taskCode` JSON file that drives the agency officer review UI. For each `taskCode` that the NSW workflow can inject, a task config defines:

- **UI metadata** — title, description, icon, and category shown in the task list and review screen header.
- **Form references** — which [forms](./forms.md) to render for the trader-submitted data view and the officer's review action.
- **Behavior** — how the officer's review outcome maps to a final application status.

Forms themselves are stored separately and referenced by ID; the same form can be reused across multiple task configs. See [`forms.md`](./forms.md) for the form file structure.

## File Location

Task configs live in `<CONFIG_DIR>/task-configs/` (default: `./data/task-configs/`). The filename (without `.json`) is the `taskCode`:

```
data/task-configs/
├── default.json                          # fallback config
└── moh:fcau:health_cert:v1.json          # taskCode: "moh:fcau:health_cert:v1"
```

At startup the `TaskConfigStore` loads every `.json` file in the directory and indexes them by basename. Resolution at request time is an O(1) map lookup.

## Schema

```json
{
  "taskCode": "fcau_general_application_v1",
  "meta": {
    "title": "General Food Export Application Review",
    "description": "Review the general application for food control administration.",
    "icon": "emoji:📋",
    "category": "Food Control"
  },
  "forms": {
    "view": "fcau_general_application_v1_view",
    "review": "fcau_general_application_v1_review"
  },
  "behavior": {
    "outcomeField": "review_outcome",
    "statusMap": {
      "approve": "APPROVED",
      "reject": "REJECTED",
      "needs_more_info": "FEEDBACK_REQUESTED"
    }
  }
}
```

| Field                    | Required | Description                                                                                                                          |
|--------------------------|----------|--------------------------------------------------------------------------------------------------------------------------------------|
| `taskCode`               | optional | Logical task code. If omitted, the filename (without `.json`) is used.                                                               |
| `meta.title`             | yes      | Display title shown in the task list and review screen header.                                                                       |
| `meta.description`       | no       | One-line description shown under the title.                                                                                          |
| `meta.icon`              | no       | Icon hint. Currently the frontend renders only `emoji:<char>`-prefixed values; other formats are ignored.                            |
| `meta.category`          | no       | Category label shown in the task list (e.g. `Food Control`).                                                                         |
| `forms.view`             | no       | Form ID for the read-only display of the trader's submitted data. Omit if the task has no trader-side data to display.               |
| `forms.review`           | no       | Form ID for the officer's review action form. Omit if there's no review action.                                                      |
| `behavior.outcomeField`  | no       | Name of the field in the review submission body whose value is looked up in `statusMap`. Defaults to `review_outcome`.               |
| `behavior.statusMap`     | no       | Maps the outcome field's value to a final application status. If absent or no key matches, status defaults to `DONE`.                |

## Resolution Flow

When `GET /api/v1/applications/{taskId}` is called:

1. The application record is loaded from the database; it carries `taskCode`.
2. The template provider resolves the task configuration by `taskCode` using `template.Provider.GetTaskConfig(taskCode)`:
   - **Hit** → returns the config.
   - **Miss** → returns an error; the response omits all metadata and form fields, and a warning is logged.
3. Each non-empty form reference in the config is resolved against the loaded form templates:
   - Hit → form JSON is attached to the response as `dataForm` (view) or `agencyForm` (review).
   - Miss → a warning is logged and the field is omitted.
4. On review submission via `POST /api/v1/applications/{taskId}/review`, if `behavior.statusMap` is set and the request body contains a matching `review_outcome` value, the application's status is set accordingly. Otherwise it defaults to `DONE`.

## Adding a New Task

1. Decide the `taskCode` that NSW will inject for this task type (e.g. `moh:fcau:health_cert:v1`).

2. Author the form file(s) under `data/forms/`. See [`forms.md`](./forms.md) for the file structure.

3. Create the task config at `data/task-configs/<taskCode>.json`:

   ```json
   {
     "taskCode": "moh:fcau:health_cert:v1",
     "meta": {
       "title": "Health Certificate Review",
       "icon": "emoji:🏥",
       "category": "Food Control"
     },
     "forms": {
       "review": "moh_fcau_health_cert_v1_review"
     },
     "behavior": {
       "statusMap": {
         "approve": "APPROVED",
         "reject":  "REJECTED"
       }
     }
   }
   ```

4. Restart the Agency service — configs and forms are loaded once at startup.

## Status Mapping

The `behavior.statusMap` field declaratively wires the officer's review action to the final application status, removing the need for hardcoded outcome logic in the service.

- The review form is expected to produce a field whose value identifies the outcome. By default this field is `review_outcome`; configure `behavior.outcomeField` to read from a different field name.
- The values that field can produce (`approve`, `reject`, `pass`, `fail`, …) are defined by the review form's schema (typically via `oneOf`).
- Each possible value should appear as a key in `statusMap`; the mapped value becomes the application's stored status.
- If `statusMap` is absent, the outcome field is missing from the submission, or the value doesn't match any key, the status defaults to `DONE`.

Common statuses used by the frontend:

| Status               | Meaning                                               |
|----------------------|-------------------------------------------------------|
| `PENDING`            | Awaiting officer review (set at injection).           |
| `APPROVED`           | Officer approved.                                     |
| `REJECTED`           | Officer rejected.                                     |
| `FEEDBACK_REQUESTED` | Officer sent the task back to the trader for changes. |
| `DONE`               | Generic completion when no `statusMap` matches.       |

Only standard task configurations ship in the repo. Agency-specific task configs live outside version control and are provided per deployment by setting `TASK_CONFIGS_DIR` and `FORM_TEMPLATES_DIR`.

## Configuration

| Variable             | Description                                                       | Default                |
|----------------------|-------------------------------------------------------------------|------------------------|
| `TASK_CONFIGS_DIR`   | Directory containing task configurations                          | `./data/task-configs`  |
| `FORM_TEMPLATES_DIR` | Directory containing form templates                               | `./data/forms`         |
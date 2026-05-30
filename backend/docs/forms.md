# Forms

Forms are [JSON Forms](https://jsonforms.io/) definitions (`schema` + `uiSchema`) that the Agency frontend renders for two purposes:

- **View forms** — read-only renderings of the trader-submitted data shown on the review screen.
- **Review forms** — interactive forms the officer fills in to record their review action.

A form file is purpose-agnostic: the same file can be referenced as a view form by one task and a review form by another. Forms are referenced by ID from [task configs](./task-configs.md) — they are not bound to a `taskCode` themselves.

## Source layering

At startup the `FormStore` loads forms from two layered sources:

1. **Primary source** (optional) — configured via the `FORMS_SOURCE_*` env vars. Either a local directory or a GitHub repo. The primary source wins on ID conflicts.
2. **Built-in defaults** (always loaded) — flat JSON files under `<CONFIG_DIR>/forms/` (default: `./data/forms/`). These are bundled with the binary and serve as the fallback layer; the shipped `default_review.json` ensures the default review screen always has a form to render even if the primary source is misconfigured.

Both layers are read into memory at startup, with each form validated as JSON. Lookups are O(1) map reads at request time.

### Built-in defaults

Form ID = filename without `.json`:

```
data/forms/
├── default_review.json                       # form ID: "default_review"
└── moh_fcau_health_cert_v1_review.json       # form ID: "moh_fcau_health_cert_v1_review"
```

### Primary source

The primary source resolves a form ID to bytes via one of:

- **Local flat** (`FORMS_SOURCE_TYPE=local`, no `manifest.json` in `FORMS_SOURCE_LOCAL_DIR`) — every `*.json` file in the directory; ID = filename.
- **Local manifest** (`FORMS_SOURCE_TYPE=local`, with `manifest.json` in `FORMS_SOURCE_LOCAL_DIR`) — IDs come from the manifest's `byId` map; files live at the relative paths it names. Used to consume a clone of e.g. `OpenNSW/one-trade-templates` directly in dev.
- **GitHub** (`FORMS_SOURCE_TYPE=github`) — `FORMS_SOURCE_GITHUB_REPO` + `FORMS_SOURCE_GITHUB_REF` resolve a `manifest.json` over raw.githubusercontent.com; blobs are fetched lazily and cached. Set `FORMS_SOURCE_GITHUB_REFRESH_INTERVAL` (e.g. `5m`) to re-poll the manifest in the background.

Set `FORMS_SOURCE_TYPE=none` (the default) to disable the primary source and rely only on built-in defaults.

## File Structure

Each form file is a top-level object with two keys: `schema` and `uiSchema`.

```json
{
  "schema": {
    "type": "object",
    "required": ["review_outcome"],
    "properties": {
      "review_outcome": {
        "type": "string",
        "title": "Review Outcome",
        "oneOf": [
          { "const": "approve", "title": "Approve" },
          { "const": "reject",  "title": "Reject" }
        ]
      },
      "rejection_reason": { "type": "string", "title": "Reason / Comments" }
    }
  },
  "uiSchema": {
    "type": "VerticalLayout",
    "elements": [
      { "type": "Control", "scope": "#/properties/review_outcome" },
      { "type": "Control", "scope": "#/properties/rejection_reason", "options": { "multi": true } }
    ]
  }
}
```

- `schema` follows standard [JSON Schema](https://json-schema.org/) and is used for both validation and field-title lookup.
- `uiSchema` follows [JSON Forms UI Schema](https://jsonforms.io/docs/uischema/) and controls layout, rules, and rendering options.

No fields are required by the Agency service itself — the form is forwarded to the frontend verbatim. Field requirements (such as `review_outcome` for status-mapping behavior) come from the task config that *references* the form, not from the form file. See [`task-configs.md`](./task-configs.md) for the contract.

## Adding a New Form

1. Create a `.json` file in `data/forms/`. The basename becomes the form ID. Use any naming convention you like; a useful one is `<taskCode>_view` or `<taskCode>_review` to make the relationship obvious.

   ```bash
   touch data/forms/moh_fcau_health_cert_v1_review.json
   ```

2. Populate it with `schema` and `uiSchema`. Validate by running `jq . data/forms/<file>.json` or pasting into any JSON Forms playground.

3. Reference it from a task config (see [`task-configs.md`](./task-configs.md)):

   ```json
   {
     "forms": { "review": "moh_fcau_health_cert_v1_review" }
   }
   ```

4. Restart the Agency service — forms are loaded once at startup.

## Per-Deployment Forms

Only `default_review.json` ships in the repo as a built-in default. Agency-specific forms are normally supplied per deployment via the **primary source** (`FORMS_SOURCE_*` env vars) — typically `OpenNSW/one-trade-templates` in production, or a local clone in dev. `CONFIG_DIR` continues to point at the directory containing the built-in defaults that always remain available as a fallback layer.

## Coherence check

At startup, after both stores have loaded, the service iterates every task config and verifies that every `forms.view` and `forms.review` ID resolves against the form store. Each missing reference is emitted as a `slog.Warn` so misconfiguration (e.g. naming-convention drift between the templates and agency-configs repos) surfaces within seconds of deployment — but the service still comes up; the affected request just gets no form.
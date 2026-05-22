# Forms

Forms are [JSON Forms](https://jsonforms.io/) definitions (`schema` + `uiSchema`) that the Agency frontend renders for two purposes:

- **View forms** — read-only renderings of the trader-submitted data shown on the review screen.
- **Review forms** — interactive forms the officer fills in to record their review action.

A form file is purpose-agnostic: the same file can be referenced as a view form by one task and a review form by another. Forms are referenced by ID from [task configs](./task-configs.md) — they are not bound to a `taskCode` themselves.

## File Location

All form files live in `<CONFIG_DIR>/forms/` (default: `./data/forms/`). The form ID is the filename without the `.json` extension:

```
data/forms/
├── default_review.json                       # form ID: "default_review"
└── moh_fcau_health_cert_v1_review.json       # form ID: "moh_fcau_health_cert_v1_review"
```

At startup, the `FormStore` reads every `.json` file in the directory, validates that it parses as JSON, and caches the raw bytes in memory. The forms are then resolvable by ID from task configs.

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

Only `default_review.json` ships in the repo. Agency-specific forms live outside version control and are provided per deployment by pointing `CONFIG_DIR` at a directory containing your `forms/` (and `task-configs/`) subdirs.
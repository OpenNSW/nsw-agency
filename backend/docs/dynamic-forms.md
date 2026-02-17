# Dynamic Forms

The OGA service uses a metadata-driven form system that determines which review form an officer sees when reviewing an application. Forms are defined as static JSON files using the [JSON Forms](https://jsonforms.io/) specification and loaded into memory at startup.

## How It Works

1. When the NSW workflow engine injects data via `POST /api/oga/inject`, it can include a `meta` object
2. When an officer fetches an application via `GET /api/oga/applications/{taskId}`, the service resolves the appropriate form and attaches it to the response
3. The frontend renders the form using a JSON Forms renderer

### Form Resolution

The form is selected by constructing a form ID from the metadata:

```
Form ID = meta.type + ":" + meta.verificationId
```

For example, if `meta` is:

```json
{
  "type": "consignment",
  "verificationId": "moa:npqs:phytosanitary:001"
}
```

The service looks for a form with ID `consignment:moa:npqs:phytosanitary:001`.

**Fallback strategy:**
1. Look up form by constructed ID
2. If not found, use the default form (configured via `OGA_DEFAULT_FORM_ID`, defaults to `"default"`)

## Form File Structure

Forms live in the `data/forms/` directory. Each file is named `{formId}.json`:

```
data/forms/
├── default.json
└── consignment:moa:npqs:phytosanitary:001.json
```

The filename (minus `.json`) becomes the form ID. At startup, the `FormStore` reads all `.json` files from this directory, validates that they contain valid JSON, and caches them in memory.

## Form Schema

Each form file contains a `schema` (JSON Schema for validation) and a `uiSchema` (JSON Forms layout):

```json
{
  "schema": {
    "type": "object",
    "required": ["decision"],
    "properties": {
      "decision": {
        "type": "string",
        "title": "Decision",
        "oneOf": [
          { "const": "APPROVED", "title": "Approved" },
          { "const": "REJECTED", "title": "Rejected" }
        ]
      },
      "remarks": {
        "type": "string",
        "title": "Remarks"
      }
    }
  },
  "uiSchema": {
    "type": "VerticalLayout",
    "elements": [
      { "type": "Control", "scope": "#/properties/decision" },
      { "type": "Control", "scope": "#/properties/remarks", "options": { "multi": true } }
    ]
  }
}
```

### Requirements

- The `decision` field **must** be present in every form schema. The review handler validates that this field exists and is non-empty in the request body.
- The `schema` follows standard [JSON Schema](https://json-schema.org/) conventions.
- The `uiSchema` follows [JSON Forms UI Schema](https://jsonforms.io/docs/uischema/) conventions.

## Adding a New Form

To add a review form for a new agency or verification type:

1. **Determine the form ID** from the metadata that the NSW workflow will send. The ID follows the pattern `{type}:{verificationId}`.

2. **Create the JSON file** in `data/forms/`:

   ```bash
   # Example for a health certificate form
   touch data/forms/consignment:moh:fcau:health_cert:001.json
   ```

3. **Define the schema and UI layout:**

   ```json
   {
     "schema": {
       "type": "object",
       "required": ["decision"],
       "properties": {
         "decision": {
           "type": "string",
           "title": "Decision",
           "oneOf": [
             { "const": "APPROVED", "title": "Approved" },
             { "const": "REJECTED", "title": "Rejected" }
           ]
         },
         "healthCertificateNumber": {
           "type": "string",
           "title": "Health Certificate Number"
         },
         "remarks": {
           "type": "string",
           "title": "Remarks"
         }
       }
     },
     "uiSchema": {
       "type": "VerticalLayout",
       "elements": [
         { "type": "Control", "scope": "#/properties/decision" },
         { "type": "Control", "scope": "#/properties/healthCertificateNumber" },
         { "type": "Control", "scope": "#/properties/remarks", "options": { "multi": true } }
       ]
     }
   }
   ```

4. **Configure the NSW workflow** to include the matching metadata in the inject request:

   ```json
   {
     "submission": {
       "url": "http://localhost:8082/api/oga/inject",
       "request": {
         "meta": {
           "type": "consignment",
           "verificationId": "moh:fcau:health_cert:001"
         }
       }
     }
   }
   ```

5. **Restart the OGA service** -- forms are loaded once at startup.

## Existing Forms

| Form ID                                  | File                                          | Description                                                              |
|------------------------------------------|-----------------------------------------------|--------------------------------------------------------------------------|
| `default`                                | `default.json`                                | Generic form with decision + remarks                                     |
| `consignment:moa:npqs:phytosanitary:001` | `consignment:moa:npqs:phytosanitary:001.json` | NPQS phytosanitary review with clearance status and inspection reference |

## Configuration

| Variable              | Description                                        | Default        |
|-----------------------|----------------------------------------------------|----------------|
| `OGA_FORMS_PATH`      | Directory to load form files from                  | `./data/forms` |
| `OGA_DEFAULT_FORM_ID` | Form ID to use when no metadata match is found     | `default`      |
| `OGA_ALLOWED_ORIGINS` | Space-separated list of allowed CORS origins       | `*`            |
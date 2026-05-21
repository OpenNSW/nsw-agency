# NSW Agency App

## Authentication configuration

This app uses Asgardeo/Thunder OIDC for sign-in.

Required environment variables:

- `VITE_BRANDING_PATH`: Path to NSW Agency branding YAML config (e.g. `./src/configs/npqs.yaml`)
- `VITE_API_BASE_URL`: NSW Agency backend API base URL (for example `http://localhost:8081`)
- `VITE_IDP_BASE_URL`: IdP base URL (for example `https://localhost:8090`)
- `VITE_IDP_CLIENT_ID`: NSW Agency-specific IdP application client id
- `VITE_APP_URL`: public URL of this NSW Agency deployment
- `VITE_IDP_SCOPES` (optional): comma-separated scopes (defaults to `openid,profile,email`)
- `VITE_IDP_PLATFORM` (optional): SDK platform (defaults to `AsgardeoV2`)

## Per-NSW Agency deployment model

Each NSW Agency deployment should use its own IdP application configuration.

Example:

- NPQS deployment
  - `VITE_BRANDING_PATH=./src/configs/npqs.yaml`
  - `VITE_IDP_CLIENT_ID=NSW_AGENCY_PORTAL_APP_NPQS`
- FCAU deployment
  - `VITE_BRANDING_PATH=./src/configs/fcau.yaml`
  - `VITE_IDP_CLIENT_ID=NSW_AGENCY_PORTAL_APP_FCAU`
- CDA deployment
  - `VITE_BRANDING_PATH=./src/configs/cda.yaml`
  - `VITE_IDP_CLIENT_ID=NSW_AGENCY_PORTAL_APP_CDA`

This allows IdP-level user access restriction per NSW Agency app registration.

## Configuration

NSW Agency instance branding and feature configuration is defined via YAML files.

### How it works

1. `src/configs/default.yaml` provides base fallback values for all instances.
2. A custom YAML file (specified via `VITE_BRANDING_PATH`) overrides specific values for each NSW Agency.
3. At build time, Vite reads the YAML files from the filesystem (at any path), merges them, and injects the result into the application.
4. The merged config is validated before the app renders.

### Adding a new NSW Agency instance

1. Create a new YAML file anywhere on your system (e.g., `./src/brand.yaml`, `../shared/npqs.yaml`, or `/etc/NSW Agency/config.yaml`).
2. Edit the `branding.appName` field (required).
3. Set `VITE_BRANDING_PATH` to the path of your YAML file in your environment.

### Config schema

```yaml
branding:
  appName: 'My NSW Agency Name' # Required
  logoUrl: '' # Optional
  favicon: '' # Optional
```

## Local development

```bash
pnpm install
pnpm run dev
```

### Running a specific NSW Agency

Use the repo-root [../run-agency.sh](../run-agency.sh) to start the frontend (and optionally the backend) with the per-agency port, branding name, API URL, and IdP client id:

```bash
# From the repo root
./run-agency.sh npqs frontend     # NPQS frontend on port 5174
./run-agency.sh fcau frontend     # FCAU frontend on port 5175
./run-agency.sh ird  frontend     # IRD  frontend on port 5176
./run-agency.sh cda  frontend     # CDA  frontend on port 5177
./run-agency.sh npqs              # also start the matching backend
```

Each name maps to a JSON file under [public/configs/](public/configs/) (`<name>.branding.json`). To onboard a new agency, copy [public/configs/default.branding.json](public/configs/default.branding.json), edit the `branding.*` fields, and add a new `case` to [../run-agency.sh](../run-agency.sh).

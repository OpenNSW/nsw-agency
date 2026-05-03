# OGA App

## Authentication configuration

This app uses Asgardeo/Thunder OIDC for sign-in.

Required environment variables:

- `VITE_BRANDING_PATH`: Path to OGA branding YAML config (e.g. `./src/configs/npqs.yaml`)
- `VITE_API_BASE_URL`: OGA backend API base URL (for example `http://localhost:8081`)
- `VITE_IDP_BASE_URL`: IdP base URL (for example `https://localhost:8090`)
- `VITE_IDP_CLIENT_ID`: OGA-specific IdP application client id
- `VITE_APP_URL`: public URL of this OGA deployment
- `VITE_IDP_SCOPES` (optional): comma-separated scopes (defaults to `openid,profile,email`)
- `VITE_IDP_PLATFORM` (optional): SDK platform (defaults to `AsgardeoV2`)

## Per-OGA deployment model

Each OGA deployment should use its own IdP application configuration.

Example:

- NPQS deployment
  - `VITE_BRANDING_PATH=./src/configs/npqs.yaml`
  - `VITE_IDP_CLIENT_ID=OGA_PORTAL_APP_NPQS`
- FCAU deployment
  - `VITE_BRANDING_PATH=./src/configs/fcau.yaml`
  - `VITE_IDP_CLIENT_ID=OGA_PORTAL_APP_FCAU`
- CDA deployment
  - `VITE_BRANDING_PATH=./src/configs/cda.yaml`
  - `VITE_IDP_CLIENT_ID=OGA_PORTAL_APP_CDA`

This allows IdP-level user access restriction per OGA app registration.

## Configuration

OGA instance branding and feature configuration is defined via YAML files.

### How it works

1. `src/configs/default.yaml` provides base fallback values for all instances.
2. A custom YAML file (specified via `VITE_BRANDING_PATH`) overrides specific values for each OGA.
3. At build time, Vite reads the YAML files from the filesystem (at any path), merges them, and injects the result into the application.
4. The merged config is validated before the app renders.

### Adding a new OGA instance

1. Create a new YAML file anywhere on your system (e.g., `./src/brand.yaml`, `../shared/npqs.yaml`, or `/etc/oga/config.yaml`).
2. Edit the `branding.appName` field (required).
3. Set `VITE_BRANDING_PATH` to the path of your YAML file in your environment.

### Config schema

```yaml
branding:
  appName: "My OGA Name"       # Required
  logoUrl: ""                   # Optional
  favicon: ""                   # Optional
```

## Local development

```bash
pnpm install
pnpm run dev
```

# Deploying NSW Agency on OpenShift

This guide describes how to deploy the NSW Agency backend and frontend to OpenShift,
including the **database migration** flow (baked into the image) and the **user/role seed**
flow (mounted dynamically at deploy time).

It is written for a per-agency deployment model (NPQS, FCAU, IRD, CDA) backed by PostgreSQL.

---

## 1. Architecture overview

Each agency runs two workloads:

| Workload             | Image                                                 | Container port                | Exposed via         |
|----------------------|-------------------------------------------------------|-------------------------------|---------------------|
| Backend (Go)         | `ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag>` | `8081` (override with `PORT`) | `Service` + `Route` |
| Frontend (Nginx SPA) | `ghcr.io/opennsw/nsw-agency/nsw-agency-app:<tag>`     | `8080`                        | `Service` + `Route` |

The backend image is OpenShift-friendly out of the box:

- Backend runs as UID `1001` (`appuser`); frontend runs as the unprivileged nginx user
  (UID `101`, group `0`) so it tolerates OpenShift's random UID.
- No privileged mode or root is required.

### What ships in the image vs. what is supplied at deploy time

| Artifact                    | Source                                        | How it reaches the pod                                                           |
|-----------------------------|-----------------------------------------------|----------------------------------------------------------------------------------|
| `agency` server binary      | `backend/Dockerfile`                          | Baked into image (`/app/agency`)                                                 |
| `migrate` CLI binary        | `backend/Dockerfile`                          | Baked into image (`/app/migrate`)                                                |
| SQL migrations              | `backend/migrations/`                         | **Baked into image** (`/app/migrations`)                                         |
| Task configs                | `backend/data/task-configs/`                  | Baked into image (`/app/data/task-configs`, set via `TASK_CONFIGS_DIR` env)      |
| Form templates              | `OpenNSW/nsw-srilanka` repo (cloned at build) | Baked into image (`/app/nsw-srilanka-configs`, set via `FORM_TEMPLATES_DIR` env) |
| `seed` CLI binary           | `backend/cmd/seed`                            | Baked into image (`/app/seed`)                                                   |
| User/role seed JSON         | `backend/data/seed/<agency>_users.json`       | **Mounted at deploy time** via ConfigMap (see §5)                                |
| All configuration / secrets | env vars                                      | `Secret` + `ConfigMap` (see §4)                                                  |

---

## 2. Build and push the backend image

The backend image bakes in three binaries (`agency`, `migrate`, `seed`), the SQL
migrations, the task configs, and the form templates (cloned from `OpenNSW/nsw-srilanka`
at build time). The seed **binary** is in the image; the seed **data** is supplied
dynamically (§5), so you can re-seed without rebuilding.

```bash
docker build -t ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag> ./backend
docker push   ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag>
```

---

## 3. Provision PostgreSQL

The backend supports `sqlite` and `postgres`. For OpenShift use PostgreSQL — pods are
ephemeral and SQLite on an emptyDir would be lost on restart.

Provision a Postgres instance (OpenShift template, operator, or an external managed DB) and
note the connection details. Each agency may use a separate database, or a single shared
database — the migrations and seed are idempotent per database.

---

## 4. Create the configuration objects

### 4.1 Secret — credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agency-backend-secret
  labels: { app: agency-backend }
type: Opaque
stringData:
  DB_PASSWORD: "<postgres-password>"
  NSW_CLIENT_SECRET: "<m2m-client-secret>"
```

### 4.2 ConfigMap — backend non-secret config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: agency-backend-config
  labels: { app: agency-backend }
data:
  # Server
  PORT: "8081"
  ALLOWED_ORIGINS: "https://agency-frontend.apps.example.com"

  # Database (PostgreSQL)
  DB_DRIVER: "postgres"
  DB_HOST: "postgresql"
  DB_PORT: "5432"
  DB_USER: "postgres"
  DB_NAME: "nsw_agency_db"
  DB_SSLMODE: "require"
  MIGRATION_DIR: "/app/migrations"

  # Data directories — the image already sets these as ENV defaults
  # (TASK_CONFIGS_DIR=/app/data/task-configs, FORM_TEMPLATES_DIR=/app/nsw-srilanka-configs).
  # Override only if you mount alternative content.
  # TASK_CONFIGS_DIR: "/app/data/task-configs"
  # FORM_TEMPLATES_DIR: "/app/nsw-srilanka-configs"

  # Inbound auth (validate JWTs from SPA / NSW)
  AUTH_JWKS_URL: "https://idp.example.com/oauth2/jwks"
  AUTH_ISSUER: "https://idp.example.com"
  AUTH_AUDIENCE: "AGENCY_API"
  AUTH_CLIENT_IDS: "<SPA_AGENCY_PORTAL>,<M2M_NSW_TO_AGENCY>"
  AUTH_EXPECTED_OU: "fcau"

  # Outbound M2M to NSW API
  NSW_API_BASE_URL: "https://nsw.example.com/api/v1"
  NSW_CLIENT_ID: "<M2M_AGENCY_TO_NSW>"
  NSW_TOKEN_URL: "https://idp.example.com/oauth2/token"
  NSW_SCOPES: "nsw:task:write,nsw:consignment:read"
```

> Do **not** set `AUTH_JWKS_INSECURE_SKIP_VERIFY` / `NSW_TOKEN_INSECURE_SKIP_VERIFY` in
> production — those are dev-only TLS-skip flags. Make sure the cluster trusts the IdP/NSW
> certificate chain instead.

### 4.3 ConfigMap — frontend runtime config

The frontend image injects env vars into `runtime-env.js` at container start, so the same
image is reconfigurable per environment/agency without a rebuild.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: agency-frontend-config
  labels: { app: agency-frontend }
data:
  VITE_BRANDING_NAME: "fcau"
  VITE_API_BASE_URL: "https://agency-backend.apps.example.com"
  VITE_IDP_BASE_URL: "https://idp.example.com"
  VITE_IDP_CLIENT_ID: "<AGENCY_PORTAL_CLIENT_ID>"
  VITE_APP_URL: "https://agency-frontend.apps.example.com"
  VITE_IDP_SCOPES: "openid,profile,email,ou,role"
  VITE_EXPECTED_OU_HANDLE: "fcau"
```

---

## 5. Migrations and seed

### 5.1 Migrations — init container (runs every rollout)

Migrations are baked into the image at `/app/migrations` and applied by the `migrate up`
command. Run it as an **init container** so the schema is up to date before the server
starts on every rollout. `migrate up` is idempotent — already-applied migrations are skipped.

This init container is defined inside the backend Deployment in §6.

### 5.2 Seed data — mount dynamically via ConfigMap

The seed JSON is **not** baked into the image — supply it at deploy time so it can change
without rebuilding. Create a ConfigMap from the agency seed file:

```bash
oc create configmap agency-seed-data \
  --from-file=fcau_users.json=backend/data/seed/fcau_users.json
```

The file format (see `backend/cmd/seed`):

```json
{
  "users": [
    { "name": "Jane Doe", "email": "jane@agency.gov.au", "roles": ["lab_officer"] }
  ]
}
```

### 5.3 Seed Job — run on demand

Seeding is a one-shot, idempotent operation (existing users are skipped), so run it as a
`Job` rather than wiring it into the pod lifecycle. Re-run it whenever the seed ConfigMap
changes.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: agency-seed
  labels: { app: agency-backend }
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: seed
          image: ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag>
          command: ["/app/seed", "user", "add", "--file", "/seed/fcau_users.json"]
          envFrom:
            - configMapRef: { name: agency-backend-config }
            - secretRef:    { name: agency-backend-secret }
          volumeMounts:
            - name: seed-data
              mountPath: /seed
              readOnly: true
      volumes:
        - name: seed-data
          configMap:
            name: agency-seed-data
```

Run it (and re-run after updating the ConfigMap):

```bash
oc apply -f seed-job.yaml
oc delete job agency-seed --ignore-not-found && oc apply -f seed-job.yaml   # to re-run
oc logs -f job/agency-seed
```

> The seed Job depends on the schema existing. Run it **after** the first backend rollout
> (whose init container applies the migrations), or add a matching `migrate up` init
> container to the Job if you want it fully standalone.

---

## 6. Backend Deployment, Service, Route

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agency-backend
  labels: { app: agency-backend }
spec:
  replicas: 2
  selector:
    matchLabels: { app: agency-backend }
  template:
    metadata:
      labels: { app: agency-backend }
    spec:
      # Apply pending migrations before the server starts. Idempotent.
      initContainers:
        - name: migrate
          image: ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag>
          command: ["/app/migrate", "up"]
          envFrom:
            - configMapRef: { name: agency-backend-config }
            - secretRef:    { name: agency-backend-secret }
      containers:
        - name: agency
          image: ghcr.io/opennsw/nsw-agency/nsw-agency-backend:<tag>
          ports:
            - containerPort: 8081
          envFrom:
            - configMapRef: { name: agency-backend-config }
            - secretRef:    { name: agency-backend-secret }
          readinessProbe:
            httpGet: { path: /health, port: 8081 }
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet: { path: /health, port: 8081 }
            initialDelaySeconds: 10
            periodSeconds: 15
---
apiVersion: v1
kind: Service
metadata:
  name: agency-backend
  labels: { app: agency-backend }
spec:
  selector: { app: agency-backend }
  ports:
    - port: 8081
      targetPort: 8081
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: agency-backend
  labels: { app: agency-backend }
spec:
  to: { kind: Service, name: agency-backend }
  port: { targetPort: 8081 }
  tls: { termination: edge }
```

> **Form templates** are baked into the image (cloned from `OpenNSW/nsw-srilanka` at build
> time into `/app/nsw-srilanka-configs`, with `FORM_TEMPLATES_DIR` set as an image `ENV`
> default), so no volume mount is needed. The backend loads them at startup and **fails
> fast** if the directory is empty — pin a known-good `nsw-srilanka` revision in the
> Dockerfile clone if you need build reproducibility.

---

## 7. Frontend Deployment, Service, Route

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agency-frontend
  labels: { app: agency-frontend }
spec:
  replicas: 2
  selector:
    matchLabels: { app: agency-frontend }
  template:
    metadata:
      labels: { app: agency-frontend }
    spec:
      containers:
        - name: app
          image: ghcr.io/opennsw/nsw-agency/nsw-agency-app:<tag>
          ports:
            - containerPort: 8080
          envFrom:
            - configMapRef: { name: agency-frontend-config }
          readinessProbe:
            httpGet: { path: /, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: agency-frontend
  labels: { app: agency-frontend }
spec:
  selector: { app: agency-frontend }
  ports:
    - port: 8080
      targetPort: 8080
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: agency-frontend
  labels: { app: agency-frontend }
spec:
  to: { kind: Service, name: agency-frontend }
  port: { targetPort: 8080 }
  tls: { termination: edge }
```

---

## 8. Deployment order

```bash
# 1. Config + secrets
oc apply -f secret.yaml
oc apply -f backend-config.yaml
oc apply -f frontend-config.yaml

# 2. Seed ConfigMap (from repo file; form templates are already baked into the image)
oc create configmap agency-seed-data \
  --from-file=fcau_users.json=backend/data/seed/fcau_users.json

# 3. Backend (init container runs `migrate up` automatically)
oc apply -f backend.yaml
oc rollout status deploy/agency-backend

# 4. Seed users/roles (after schema exists)
oc apply -f seed-job.yaml
oc logs -f job/agency-seed

# 5. Frontend
oc apply -f frontend.yaml
oc rollout status deploy/agency-frontend
```

---

## 9. Per-agency matrix

Deploy the same images per agency, changing only configuration:

| Setting | NPQS | FCAU | CDA | SLPA |
| --- | --- | --- | --- | --- |
| `AUTH_EXPECTED_OU` / `VITE_EXPECTED_OU_HANDLE` | `npqs` | `fcau` | `cda` | `slpa` |
| `VITE_BRANDING_NAME` | `npqs` | `fcau` | `cda` | `slpa` |
| Seed file | `npqs_users.json` | `fcau_users.json` | `cda_users.json` | `slpa_users.json` |
| `NSW_CLIENT_ID` / `VITE_IDP_CLIENT_ID` | agency-specific | agency-specific | agency-specific | agency-specific |

Use a separate namespace (or a name suffix) per agency, e.g. `agency-backend-fcau`.

---

## 10. Verification

```bash
# Migrations applied
oc logs deploy/agency-backend -c migrate

# Backend health
oc exec deploy/agency-backend -- wget -qO- localhost:8081/health

# Seeded users (check Job output)
oc logs job/agency-seed   # → "seed: successfully seeded N user(s)"

# Routes
oc get route agency-backend agency-frontend
```

---

## 11. Operational notes

- **Re-running migrations:** every backend rollout runs `migrate up` via the init
  container; it is a no-op when there is nothing pending. Roll back the last migration
  manually with a one-off pod: `oc run migrate-down --rm -it --restart=Never
  --image=<backend-image> --command -- /app/migrate down` (wire in the same env).
- **Re-seeding:** update `agency-seed-data` ConfigMap, then delete and re-apply the seed
  Job. Existing users are skipped, so it is safe to re-run.
- **Secrets:** keep `DB_PASSWORD` and `NSW_CLIENT_SECRET` only in the `Secret`. Never put
  them in the ConfigMap or image.
- **Scaling:** the backend is stateless when using PostgreSQL, so `replicas` can be raised
  freely. Avoid SQLite (`DB_DRIVER=sqlite`) on OpenShift — it does not survive pod restarts
  and cannot be shared across replicas.

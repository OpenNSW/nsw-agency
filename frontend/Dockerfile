FROM node:22-bookworm-slim AS builder

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
WORKDIR /workspace

RUN corepack enable

# Copy workspace manifests
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/trader-app/package.json apps/trader-app/package.json
COPY apps/oga-app/package.json apps/oga-app/package.json
COPY packages/jsonforms-renderers/package.json packages/jsonforms-renderers/package.json
COPY packages/ui/package.json packages/ui/package.json

# Optimized install (combined fetch/install to save local disk layers)
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile

COPY . .

# Build including dependencies
RUN pnpm --filter oga-app... build

FROM nginxinc/nginx-unprivileged:1.27-alpine

LABEL org.opencontainers.image.source="https://github.com/OpenNSW/nsw"
LABEL org.opencontainers.image.description="NSW OGA Officer Portal Frontend"

COPY apps/oga-app/nginx.conf /etc/nginx/templates/default.conf.template
COPY --from=builder /workspace/apps/oga-app/dist /usr/share/nginx/html

# OpenShift Compliance: Fix permissions for random UID (Group 0)
USER root
COPY --chown=101:0 apps/oga-app/docker-entrypoint.sh /docker-entrypoint.d/40-runtime-env.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-env.sh && \
    chmod -R g+w /var/cache/nginx /var/run /var/log/nginx /etc/nginx/conf.d /usr/share/nginx/html
USER 101
EXPOSE 8080


CMD ["nginx", "-g", "daemon off;"]
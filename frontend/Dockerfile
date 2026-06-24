FROM node:22-bookworm-slim AS builder

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./

RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile

COPY . .

ARG BUILD_VERSION=dev
ENV VITE_APP_VERSION=${BUILD_VERSION}

RUN pnpm build

FROM nginxinc/nginx-unprivileged:stable-alpine3.23-perl

LABEL org.opencontainers.image.source="https://github.com/OpenNSW/nsw-agency"
LABEL org.opencontainers.image.description="NSW Agency Officer Portal Frontend"

COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY --from=builder /app/dist /usr/share/nginx/html

# OpenShift Compliance: Fix permissions for random UID (Group 0)
USER root
COPY --chown=101:0 docker-entrypoint.sh /docker-entrypoint.d/40-runtime-env.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-env.sh && \
    chmod -R g+w /var/cache/nginx /var/run /var/log/nginx /etc/nginx/conf.d /usr/share/nginx/html
USER 101
EXPOSE 8080


CMD ["nginx", "-g", "daemon off;"]

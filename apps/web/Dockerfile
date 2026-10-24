# syntax=docker/dockerfile:1.7
# Build context is the repository root (pnpm workspace). See compose.yaml.
# Ignore rules: Dockerfile.dockerignore next to this file.
FROM node:24-alpine AS base
ENV PNPM_HOME=/pnpm PATH=/pnpm:$PATH NEXT_TELEMETRY_DISABLED=1
RUN corepack enable

FROM base AS build
WORKDIR /repo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/content-schema/package.json packages/content-schema/
COPY packages/api-client/package.json packages/api-client/
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --ignore-scripts --filter @kubedojo/web...
COPY packages/api-client packages/api-client
COPY apps/web apps/web
ARG VERSION=dev
ARG COMMIT=unknown
RUN pnpm --filter @kubedojo/web build

FROM node:24-alpine AS runtime
WORKDIR /app
ARG VERSION=dev
ARG COMMIT=unknown
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0 \
    KUBEDOJO_VERSION=$VERSION KUBEDOJO_COMMIT=$COMMIT
COPY --from=build --chown=node:node /repo/apps/web/.next/standalone ./
COPY --from=build --chown=node:node /repo/apps/web/.next/static ./apps/web/.next/static
COPY --from=build --chown=node:node /repo/apps/web/public ./apps/web/public
USER node
EXPOSE 3000
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD ["node", "-e", "fetch('http://127.0.0.1:3000/api/healthz').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
CMD ["node", "apps/web/server.js"]

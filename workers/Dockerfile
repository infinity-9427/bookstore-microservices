# workers/Dockerfile - Optimized multi-stage build
FROM node:22-alpine AS base
WORKDIR /app
RUN corepack enable && \
    # Cache pnpm globally to avoid re-downloading
    corepack use pnpm@latest

# 1) Install dependencies with better caching
FROM base AS deps
COPY package.json pnpm-lock.yaml ./
# Use cache mounts for faster rebuilds
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile && \
    # Create production node_modules in separate location
    mkdir -p /tmp/prod && \
    cp package.json pnpm-lock.yaml /tmp/prod/ && \
    cd /tmp/prod && \
    pnpm install --frozen-lockfile --prod

# 2) Build TypeScript (reuse deps layer)
FROM deps AS build
COPY tsconfig.json ./
COPY src ./src
RUN pnpm build

# 3) Final runtime image - single stage copy
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production

# Copy only what we need for runtime
COPY --from=build /app/dist ./dist
COPY --from=deps /tmp/prod/node_modules ./node_modules
COPY --from=deps /tmp/prod/package.json ./package.json

# Use non-root user for security
RUN addgroup -g 1001 -S nodejs && \
    adduser -S worker -u 1001
USER worker

CMD ["node", "dist/worker.js"]

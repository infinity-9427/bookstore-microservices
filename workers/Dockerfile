# workers/Dockerfile
FROM node:22-alpine AS base
WORKDIR /app
RUN corepack enable   # enables pnpm

# 1) Install all deps (incl. dev) to build
FROM base AS deps
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# 2) Build TypeScript -> dist/
FROM base AS build
COPY --from=deps /app/node_modules ./node_modules
COPY tsconfig.json ./
COPY src ./src
RUN pnpm build      # expects "build": "tsc -p tsconfig.json"

# 3) Install only production deps
FROM base AS prod-deps
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --prod

# 4) Runtime image
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/dist ./dist
COPY --from=prod-deps /app/node_modules ./node_modules
CMD ["node", "dist/worker.js"]

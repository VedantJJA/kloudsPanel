FROM node:20-alpine AS builder

WORKDIR /app
COPY pnpm-lock.yaml package.json ./
RUN npm install -g pnpm && pnpm fetch

COPY . .
RUN pnpm install -r --offline
RUN pnpm --filter web build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/apps/web/build ./build
COPY --from=builder /app/apps/web/package.json ./
COPY --from=builder /app/node_modules ./node_modules
# We need production dependencies. A simpler approach for the adapter-node is just running the build index.js.
EXPOSE 3000
CMD ["node", "build/index.js"]

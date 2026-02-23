# Stage 1: Build dashboard
FROM node:20-alpine AS dashboard-build
RUN corepack enable && corepack prepare pnpm@9.15.4 --activate
WORKDIR /app
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml ./
COPY apps/dashboard/package.json apps/dashboard/
RUN pnpm install --frozen-lockfile --filter dashboard...
COPY apps/dashboard/ apps/dashboard/
RUN pnpm --filter dashboard build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-build
RUN apk add --no-cache git
WORKDIR /app
COPY apps/vgx/go.mod apps/vgx/go.sum ./
RUN go mod download
COPY apps/vgx/ .
COPY --from=dashboard-build /app/apps/dashboard/dist ./internal/embed/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /vgx ./cmd/vgx

# Stage 3: Runtime (debian-slim for glibc compatibility with ast-grep)
FROM debian:bookworm-slim
ARG AST_GREP_VERSION=0.40.5
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates git grep curl unzip && \
    curl -fsSL "https://github.com/ast-grep/ast-grep/releases/download/${AST_GREP_VERSION}/app-x86_64-unknown-linux-gnu.zip" -o /tmp/sg.zip && \
    unzip /tmp/sg.zip -d /tmp/sg-extract && \
    mv /tmp/sg-extract/ast-grep /usr/local/bin/ast-grep && \
    mv /tmp/sg-extract/sg /usr/local/bin/sg && \
    chmod +x /usr/local/bin/ast-grep /usr/local/bin/sg && \
    rm -rf /tmp/sg.zip /tmp/sg-extract && \
    apt-get remove -y curl unzip && apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*
COPY --from=go-build /vgx /usr/local/bin/vgx
COPY apps/vgx/db/migrations /migrations
EXPOSE 8080
ENTRYPOINT ["vgx"]
CMD ["serve", "--port", "8080"]

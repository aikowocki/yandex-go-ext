# syntax=docker/dockerfile:1

# ─── shared builder base ──────────────────────────────────────────────────────
FROM golang:1.26-alpine AS base
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .

# ─── compile all binaries (shared go-build cache) ────────────────────────────
FROM base AS build-server
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server

FROM base AS build-worker
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker

FROM base AS build-cronjob
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/cronjob ./cmd/cronjob

FROM base AS build-migrate
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate

# ─── runtime: server ─────────────────────────────────────────────────────────
FROM alpine:3.22 AS server
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build-server --chown=app:app /out/server ./server
COPY --from=build-server --chown=app:app /src/configs ./configs
COPY --from=build-server --chown=app:app /src/web ./web
COPY --from=build-server --chown=app:app /src/docs ./docs
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]

# ─── runtime: worker ─────────────────────────────────────────────────────────
FROM alpine:3.22 AS worker
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build-worker --chown=app:app /out/worker ./worker
COPY --from=build-worker --chown=app:app /src/configs ./configs
USER app
ENTRYPOINT ["/app/worker"]

# ─── runtime: cronjob ────────────────────────────────────────────────────────
FROM alpine:3.22 AS cronjob
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build-cronjob --chown=app:app /out/cronjob ./cronjob
COPY --from=build-cronjob --chown=app:app /src/configs ./configs
COPY --chown=app:app deployments/docker/cronjob-scheduler.sh ./cronjob-scheduler.sh
COPY --chown=app:app deployments/docker/cronjob.crontab /etc/crontabs/app
RUN chmod 0555 /app/cronjob-scheduler.sh
USER app
ENTRYPOINT ["/app/cronjob"]

# ─── runtime: migrate ────────────────────────────────────────────────────────
FROM alpine:3.22 AS migrate
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build-migrate --chown=app:app /out/migrate ./migrate
COPY --from=build-migrate --chown=app:app /src/configs ./configs
USER app
ENTRYPOINT ["/app/migrate"]

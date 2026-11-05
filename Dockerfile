# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG BUILD_ENV=stage
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.BuildEnv=${BUILD_ENV}'" -o /out/fairchild-admin-panel-be cmd/main.go
# The migrator, with the migrations compiled in (cmd/migrate/main.go).
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.BuildEnv=${BUILD_ENV}'" -o /out/migrate ./cmd/migrate

# Runtime stage - tzdata for Asia/Manila month boundaries, CA certs for TLS
FROM alpine:3.22
RUN apk add --no-cache tzdata ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=build /out/fairchild-admin-panel-be /app/fairchild-admin-panel-be
COPY --from=build /out/migrate /app/migrate
# Config comes from env vars or ENV_FILE (a mounted secret); MYSQL_*_CA_CERT
# may point at a cert mounted alongside it.
USER app
EXPOSE 8081
# Apply pending migrations (the same as `make migrate-up-<env>`) before
# starting; if they fail the container exits instead of serving against an
# out-of-date schema. Never runs `down`.
# CMD ["/bin/sh", "-c", "/app/migrate up && exec /app/fairchild-admin-panel-be"]

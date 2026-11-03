# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG BUILD_ENV=stage
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.BuildEnv=${BUILD_ENV}'" -o /out/fairchild-admin-panel-be cmd/main.go

# Runtime stage - tzdata for Asia/Manila month boundaries, CA certs for TLS
FROM alpine:3.22
RUN apk add --no-cache tzdata ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=build /out/fairchild-admin-panel-be /app/fairchild-admin-panel-be
# Config comes from env vars or ENV_FILE (a mounted secret); MYSQL_*_CA_CERT
# may point at a cert mounted alongside it.
USER app
EXPOSE 8081
CMD ["/app/fairchild-admin-panel-be"]

# syntax=docker/dockerfile:1

# 1. Build the web app (web/dist).
FROM node:26.13.0-trixie-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Build the Go binary with the web app embedded (-tags webdist).
FROM golang:1.27.3-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
# REVISION is the commit being built, which CI passes in: the build context has
# no .git for go build to read it from. GET /api/health reports it.
ARG REVISION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags webdist -trimpath -ldflags="-s -w -X main.stampedRevision=${REVISION}" \
    -o /out/hoardboard ./cmd/hoardboard

# 3. Runtime: distroless static (no shell, no package manager), as root.
#
# Root on purpose: the container is meant to run with every capability
# dropped except CAP_DAC_READ_SEARCH (see AGENTS.md), which lets it list and
# stat root-only folders and nothing more. Without CAP_DAC_OVERRIDE, root can
# write only to folders root owns, such as the /data volume Docker creates.
FROM gcr.io/distroless/static-debian13@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f AS runtime
COPY --from=build /out/hoardboard /usr/local/bin/hoardboard
ENV HOARDBOARD_MOUNT_ROOT=/mnt \
    HOARDBOARD_STATUS_DIR=/var/local/emhttp \
    HOARDBOARD_DATA_DIR=/data \
    HOARDBOARD_PORT=8098 \
    HOARDBOARD_SCAN_INTERVAL=1h
EXPOSE 8098
VOLUME /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --start-interval=2s --retries=3 \
  CMD ["hoardboard", "healthcheck"]
ENTRYPOINT ["hoardboard"]

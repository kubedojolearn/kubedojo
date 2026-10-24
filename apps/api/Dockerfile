# syntax=docker/dockerfile:1.7
# Builds both Go binaries (api, worker) into a distroless, non-root image.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    LDFLAGS="-s -w \
      -X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Version=${VERSION} \
      -X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Commit=${COMMIT}" && \
    go build -trimpath -ldflags "$LDFLAGS" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags "$LDFLAGS" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /out/worker /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD ["/usr/local/bin/api", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/api"]
CMD ["serve"]

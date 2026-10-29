# We use the latest Go 1.x version unless asked to use something else.
# The GitHub Actions CI job sets this argument for a consistent Go version.
ARG GO_VERSION=1
ARG BASE_IMAGE=cgr.dev/chainguard/glibc-dynamic:latest-dev@sha256:c35cdd53eb8d848a697f233fdcc020598bb11d83a9cf63345a5a6787eb6b6546

# Setup the base environment. The BUILDPLATFORM is set automatically by Docker.
# The --platform=${BUILDPLATFORM} flag tells Docker to build the function using
# the OS and architecture of the host running the build, not the OS and
# architecture that we're building the function for.
FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS build

WORKDIR /fn

# Most functions don't want or need CGo support, so we disable it.
ENV CGO_ENABLED=0

# We run go mod download in a separate step so that we can cache its results.
# This lets us avoid re-downloading modules if we don't need to. The type=target
# mount tells Docker to mount the current directory read-only in the WORKDIR.
# The type=cache mount tells Docker to cache the Go modules cache across builds.
RUN --mount=target=. --mount=type=cache,target=/go/pkg/mod go mod download

# The TARGETOS and TARGETARCH args are set by docker. We set GOOS and GOARCH to
# these values to ask Go to compile a binary for these architectures. If
# TARGETOS and TARGETOS are different from BUILDPLATFORM, Go will cross compile
# for us (e.g. compile a linux/amd64 binary on a linux/arm64 build machine).
ARG TARGETOS
ARG TARGETARCH

# Build the function binary. The type=target mount tells Docker to mount the
# current directory read-only in the WORKDIR. The type=cache mount tells Docker
# to cache the Go modules cache across builds.
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-w -s" -o /function .

# The image carries none of the build tools used in the build stages.
# Chainguard publishes no versioned tags on its free tier, so BASE_IMAGE keeps
# a digest beside the tag and Renovate moves that digest.
FROM ${BASE_IMAGE} AS image

# The base carries no ssh. Only ssh:// KCL sources need it, and installing a
# package needs root, so the image's own non-root user is restored afterwards.
USER root
RUN apk add --no-cache openssh-client
USER 65532

WORKDIR /
COPY --from=build /function /function

# Crossplane runs functions as uid 2000, which has no passwd entry in the base,
# so HOME is '/' and KCL's library and package caches have to be pointed at a
# writable directory.
ENV KCL_LIB_HOME=/tmp KCL_PKG_PATH=/tmp

RUN /function --help
EXPOSE 9443
ENTRYPOINT ["/function"]

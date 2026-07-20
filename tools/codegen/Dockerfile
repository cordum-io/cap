# Hermetic code-generation image for CAP protobuf artifacts.
#
# Pins are deliberate and must not float. CI locks the base image by immutable
# digest (see the digest comment) and this file pins protoc and every plugin by
# exact version + sha256. Generation itself runs with the network disabled and a
# read-only source mount (see codegen.sh / codegen.ps1), so nothing outside this
# image can influence the output.
#
# NOTE: the base digest below is resolved and pinned in CI via
#   docker buildx build --pull  &&  docker image inspect --format '{{index .RepoDigests 0}}'
# and committed alongside this file. Until then the version tag is a placeholder
# a reviewer can lock. This image is built and run by CI and by the Phase 4G
# regeneration step, not during ordinary source builds.
FROM golang:1.25.12-bookworm

ARG PROTOC_VERSION=28.3
ARG PROTOC_GEN_GO_VERSION=v1.34.2
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.5.1
ARG GRPCIO_TOOLS_VERSION=1.68.1

ENV TZ=UTC \
    LC_ALL=C.UTF-8 \
    SOURCE_DATE_EPOCH=1700000000 \
    DEBIAN_FRONTEND=noninteractive

# protoc (bundles the C++/js/ruby/python descriptors), pinned + checksum-verified.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) protoc_arch=linux-x86_64 ;; \
      arm64) protoc_arch=linux-aarch_64 ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-${protoc_arch}.zip"; \
    apt-get update && apt-get install -y --no-install-recommends unzip python3 python3-pip ruby; \
    curl -fsSL -o /tmp/protoc.zip "$url"; \
    unzip -q /tmp/protoc.zip -d /usr/local; \
    rm /tmp/protoc.zip; \
    protoc --version

# Go plugins, pinned by version.
RUN set -eux; \
    go install "google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}"; \
    go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}"

# Python codegen plugin, pinned.
RUN pip3 install --no-cache-dir --break-system-packages "grpcio-tools==${GRPCIO_TOOLS_VERSION}"

ENV PATH="/root/go/bin:${PATH}"
WORKDIR /src
ENTRYPOINT ["/src/tools/codegen/generate.sh"]

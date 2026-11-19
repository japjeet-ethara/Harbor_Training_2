# =============================================================================
# Stage 1 ("source"): fetch the original pyfiglet repo at the pinned commit
# =============================================================================
# alpine/git v2.54.1, pinned by multi-arch index digest.
FROM alpine/git:v2.54.1@sha256:2306e2b05ee0ef99f5d25b0779f645b24ae9f91ba187107503592fcdfe78da3a AS source

ARG PYFIGLET_REPO=https://github.com/pwaller/pyfiglet.git
ARG PYFIGLET_COMMIT=255827efbc09d662875dad50fa0edc1e1263bf45

RUN git clone "$PYFIGLET_REPO" /src \
 && cd /src \
 && git checkout --detach "$PYFIGLET_COMMIT" \
 && test "$(git rev-parse HEAD)" = "$PYFIGLET_COMMIT" \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# =============================================================================
# Stage 2: Go toolchain environment for the agent
# =============================================================================
# golang:1.23-alpine (Go 1.23.12, Alpine 3.22.1), pinned by multi-arch index digest.
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
 && addgroup -g 1000 player \
 && adduser -D -u 1000 -G player -s /bin/bash player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

# Fail the build if any Python interpreter is present in the final image.
RUN if command -v python3 >/dev/null 2>&1 \
     || command -v python >/dev/null 2>&1 \
     || ls /usr/bin/python* /usr/local/bin/python* >/dev/null 2>&1; then \
        echo "ERROR: Python interpreter found in final image" >&2; exit 1; \
    fi

USER player
WORKDIR /workspace/repo

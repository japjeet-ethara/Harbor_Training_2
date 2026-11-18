# figlet-go: two-stage build.
#   Stage 1 "source": fetch pyfiglet at a pinned commit and finish its own setup.
#   Stage 2: the agent's room. Go + bash only, no Python, non-root user "player".
#
# Both stages use the same image, pinned by tag AND multi-arch digest (D5).
# golang:1.23.12-alpine3.22 -> sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

ARG GO_IMAGE=golang:1.23.12-alpine3.22@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

# ---------------------------------------------------------------- stage 1 (D1)
FROM ${GO_IMAGE} AS source

ARG PYFIGLET_REPO=https://github.com/pwaller/pyfiglet
ARG PYFIGLET_SHA=255827efbc09d662875dad50fa0edc1e1263bf45

RUN apk add --no-cache git

RUN git clone --quiet "${PYFIGLET_REPO}" /src \
 && cd /src \
 && git checkout --quiet "${PYFIGLET_SHA}" \
 && test "$(git rev-parse HEAD)" = "${PYFIGLET_SHA}" \
 && rm -rf .git \
 && rm -rf pyfiglet/fonts-contrib test-fonts

# D3: the project's own setup step (Makefile targets "clean" + "minimal"):
#   rm -rf pyfiglet/fonts/ ; mkdir pyfiglet/fonts/ ; cp pyfiglet/fonts-standard/* pyfiglet/fonts
RUN cd /src \
 && rm -rf pyfiglet/fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/ \
 && test -f pyfiglet/fonts/standard.flf

# ---------------------------------------------------------------- stage 2
FROM ${GO_IMAGE}

# D6: Harbor runs its commands with bash; Alpine does not ship it.
RUN apk add --no-cache bash

# D4: normal user "player", uid 1000.
RUN addgroup -g 1000 player \
 && adduser -D -u 1000 -G player -s /bin/bash player

# Only the prepared source tree crosses over from stage 1 (no git, no tools).
COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-buildvcs=false

# D2: fail the build if any Python slipped in.
RUN if command -v python3 || command -v python; then echo "python found" >&2; exit 1; fi

USER player
WORKDIR /workspace/repo

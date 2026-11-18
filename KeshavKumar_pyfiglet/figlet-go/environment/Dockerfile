# ---------------------------------------------------------------------------
# Stage 1 ("source"): fetch pyfiglet at a pinned commit and finish its setup.
# Nothing from this stage reaches the agent except /src.
# ---------------------------------------------------------------------------
# alpine/git:2.47.2, pinned by digest (D5)
FROM alpine/git@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source

ARG PYFIGLET_COMMIT=255827efbc09d662875dad50fa0edc1e1263bf45

RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout --quiet "$PYFIGLET_COMMIT" \
 && test "$(git rev-parse HEAD)" = "$PYFIGLET_COMMIT" \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 # D3: the Makefile's "clean" + "minimal" font setup; the code loads
 # fonts from pyfiglet/fonts/, but the repo ships them in fonts-standard/.
 && rm -rf pyfiglet/fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go + bash, no Python, non-root user "player".
# ---------------------------------------------------------------------------
# golang:1.23-alpine (Go 1.23.12), pinned by digest (D5).
# Not golang:1.23-bookworm: that one ships /usr/bin/python3 (D2).
FROM golang@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

# D6: Harbor runs its commands with bash; Alpine has none by default.
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

# D1: only the prepared source tree comes over from stage 1.
# D4: owned by player.
COPY --from=source --chown=player:player /src /workspace/repo

USER player
WORKDIR /workspace/repo

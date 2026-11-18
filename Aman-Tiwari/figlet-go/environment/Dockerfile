# ---------------------------------------------------------------------------
# Stage 1 ("source"): fetch pyfiglet at a pinned commit and finish its setup.
# git and everything else in this stage never reaches the agent's room.
# ---------------------------------------------------------------------------
FROM alpine/git:v2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source

ARG PYFIGLET_COMMIT=255827efbc09d662875dad50fa0edc1e1263bf45

RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout "$PYFIGLET_COMMIT" \
 && rm -rf .git \
 && rm -rf pyfiglet/fonts-contrib test-fonts \
 # Font setup from the project's Makefile ("make minimal"):
 # the code loads fonts from pyfiglet/fonts/, but they live in fonts-standard/.
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go only - no Python anywhere.
# (golang:*-bookworm ships python3, so we use Alpine and add bash for Harbor.)
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

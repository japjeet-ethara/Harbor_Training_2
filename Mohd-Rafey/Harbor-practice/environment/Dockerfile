# ---------- Stage 1: fetch pyfiglet source at a fixed commit ----------
FROM alpine/git:v2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source

ARG PYFIGLET_REPO=https://github.com/pwaller/pyfiglet
ARG PYFIGLET_SHA=255827efbc09d662875dad50fa0edc1e1263bf45

RUN git clone "$PYFIGLET_REPO" /src \
 && cd /src \
 && git checkout "$PYFIGLET_SHA" \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------- Stage 2: the agent's room (Go, no Python) ----------
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

# ---- Stage 1: fetch pyfiglet at a pinned commit and finish its own setup (Makefile font step) ----
FROM alpine:3.20.3 AS source
RUN apk add --no-cache git
ARG COMMIT=255827efbc09d662875dad50fa0edc1e1263bf45
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout "$COMMIT" \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---- Stage 2: the agent's room. No python, no git. Alpine Go + bash, normal user ----
FROM golang:1.23-alpine3.20
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace/repo \
 && chown player:player /workspace/repo
COPY --from=source --chown=player:player /src/ /workspace/repo/
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo

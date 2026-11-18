# ---------------------------------------------------------------------------
# Stage 1 "source": fetch pyfiglet at a pinned commit and finish the project's
# own setup step. git lives ONLY in this stage and never reaches the agent.
# D5: pinned tag. For a digest pin run:
#   docker inspect --format '{{index .RepoDigests 0}}' alpine:3.22
# and use  FROM alpine:3.22@sha256:...
# ---------------------------------------------------------------------------
FROM alpine:3.22 AS source
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go on Alpine = no Python (D2); bash added (D6);
# normal user "player" (D4).
# Digest pin: docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine3.22
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine3.22
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace \
 && chown player:player /workspace
COPY --from=source --chown=player:player /src /workspace/repo
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo

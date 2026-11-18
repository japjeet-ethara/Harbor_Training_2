# syntax=docker/dockerfile:1
#
# Two-stage room for the figlet-go task.
#   D1 two stages          D2 no Python in the final image     D3 font setup done
#   D4 user "player"       D5 pinned images                    D6 bash is installed

# ---------------------------------------------------------------------------
# Stage 1 "source": fetch pyfiglet at the pinned commit and finish the
# project's own setup. git and friends stay in this stage; they never reach
# the agent's room.
# Pin by digest for an exact room:  FROM alpine:3.20@sha256:<digest>
#   (docker inspect --format '{{index .RepoDigests 0}}' alpine:3.20)
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS source
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
#   ^ the Makefile's "minimal" step: the code loads fonts from pyfiglet/fonts/,
#     but the repository only ships them in pyfiglet/fonts-standard/.

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go on Alpine has NO Python (the bookworm image
# does). Alpine has no bash, and Harbor runs its commands with bash, so add it.
# Pin by digest for an exact room:  FROM golang:1.23-alpine@sha256:<digest>
#   (docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine)
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine

RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace/repo \
 && chown -R player:player /workspace

COPY --from=source --chown=player:player /src/ /workspace/repo/

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

# ---------------------------------------------------------------------------
# Stage 1 "source": fetch pyfiglet and finish the project's own setup step.
# git and friends live ONLY in this stage; they never reach the agent's room.
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS source
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 # The Makefile's "minimal" target: the code loads fonts from pyfiglet/fonts,
 # but the repo ships them in pyfiglet/fonts-standard.
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# ---------------------------------------------------------------------------
# Stage 2: the agent's room. Go on Alpine (NO Python), bash for Harbor,
# and a normal user so the agent cannot `apk add python3`.
# Pin by digest for a byte-for-byte identical room:
#   docker inspect --format '{{index .RepoDigests 0}}' golang:1.23-alpine
#   then: FROM golang:1.23-alpine@sha256:<digest>
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player \
 && mkdir -p /workspace
COPY --from=source --chown=player:player /src /workspace/repo
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo

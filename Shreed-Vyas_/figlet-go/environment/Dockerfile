# syntax=docker/dockerfile:1
# ---------- Stage 1: fetch pinned pyfiglet + set up fonts ----------
FROM alpine:3.20.3 AS fetch
RUN apk add --no-cache git
RUN git clone https://github.com/pwaller/pyfiglet /src \
 && cd /src \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && git log -1 --format='%H %cI' > /src/COMMIT.txt \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/ \
 && cp pyfiglet/fonts-contrib/shadow.flf pyfiglet/fonts/shadow.flf \
 && rm -rf /src/.git

# ---------- Stage 2: Go-only runtime (no Python) ----------
FROM golang:1.23.4-alpine3.21
RUN apk add --no-cache bash \
 && adduser -D -u 1000 -s /bin/bash player
ENV GOTOOLCHAIN=local \
    GOFLAGS=-buildvcs=false
WORKDIR /workspace/repo
COPY --from=fetch /src/pyfiglet ./pyfiglet
COPY --from=fetch /src/LICENSE ./LICENSE
COPY --from=fetch /src/COMMIT.txt ./COMMIT.txt
# visible.jsonl* is optional (matches nothing before gen.py has run); Dockerfile itself always matches.
COPY Dockerfile visible.jsonl* ./examples/
RUN mkdir -p /workspace/repo/figlet /logs/agent /logs/verifier \
 && chown -R player:player /workspace /home/player \
 && chmod -R 777 /logs
USER player

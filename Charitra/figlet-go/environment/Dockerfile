# Stage 1: fetch the exact upstream source and perform its font setup.
FROM alpine:3.20 AS source
RUN apk add --no-cache git ca-certificates
WORKDIR /src
RUN git clone https://github.com/pwaller/pyfiglet.git . \
 && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
 && rm -rf .git pyfiglet/fonts-contrib test-fonts \
 && mkdir -p pyfiglet/fonts \
 && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

# Stage 2: only Go, bash, and the prepared source tree enter the agent room.
FROM golang:1.23-alpine
RUN apk add --no-cache bash \
 && adduser -D -u 1000 player
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local
COPY --from=source --chown=player:player /src /workspace/repo
USER player
WORKDIR /workspace/repo

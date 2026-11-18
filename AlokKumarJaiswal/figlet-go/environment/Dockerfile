FROM alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 AS source

RUN apk add --no-cache git ca-certificates && \
    git clone https://github.com/pwaller/pyfiglet /src && \
    cd /src && \
    git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 && \
    [ "$(git rev-parse HEAD)" = "255827efbc09d662875dad50fa0edc1e1263bf45" ] && \
    rm -rf /src/.git /src/pyfiglet/fonts-contrib /src/test-fonts && \
    mkdir -p /src/pyfiglet/fonts && \
    cp -r /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash && \
    addgroup -g 1000 player && \
    adduser -u 1000 -G player -D -s /bin/bash player && \
    mkdir -p /workspace/repo && \
    chown -R player:player /workspace

COPY --from=source --chown=player:player /src/ /workspace/repo/

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

WORKDIR /workspace/repo

USER player

FROM alpine/git:v2.45.2@sha256:16ad8e788e1d3b0c30f18da8dde5c0ace3b187445a62d8af893b003ca1e70592 AS source

RUN git clone https://github.com/pwaller/pyfiglet /src \
    && cd /src \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp -a pyfiglet/fonts-standard/. pyfiglet/fonts/

FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
    && adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

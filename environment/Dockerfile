FROM alpine/git:v2.47.2 AS source

RUN git clone https://github.com/pwaller/pyfiglet.git /src \
    && cd /src \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf /src/.git /src/pyfiglet/fonts-contrib /src/test-fonts \
    && mkdir -p /src/pyfiglet/fonts \
    && cp -a /src/pyfiglet/fonts-standard/. /src/pyfiglet/fonts/

FROM golang:1.23.12-alpine3.21

RUN apk add --no-cache bash \
    && addgroup -g 1000 player \
    && adduser -D -u 1000 -G player player

COPY --from=source --chown=player:player /src /workspace/repo
RUN mkdir -p /tests/data \
    && touch /tests/data/visible.jsonl \
    && chown -R player:player /tests

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

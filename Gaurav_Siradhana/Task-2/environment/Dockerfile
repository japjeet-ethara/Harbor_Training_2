FROM alpine/git:v2.45.2 AS source

RUN git clone https://github.com/pwaller/pyfiglet.git /src \
    && cd /src \
    && git checkout --detach 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp -R pyfiglet/fonts-standard/. pyfiglet/fonts/

FROM golang:1.23.4-alpine3.20

RUN apk add --no-cache bash \
    && addgroup -g 1000 player \
    && adduser -D -u 1000 -G player player \
    && mkdir -p /workspace/repo \
    && chown player:player /workspace/repo

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

FROM alpine:3.20 AS source

RUN apk add git \
    && git clone https://github.com/pwaller/pyfiglet /src \
    && git -C /src checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf /src/.git \
    && rm -rf /src/pyfiglet/fonts-contrib /src/test-fonts \
    && mkdir -p /src/pyfiglet/fonts \
    && cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

FROM golang:1.23-alpine

RUN apk add --no-cache bash \
    && adduser -D -u 1000 player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 GOTOOLCHAIN=local

RUN ln -sf /usr/local/go/bin/go /usr/local/bin/go && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
USER player
WORKDIR /workspace/repo

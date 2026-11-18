FROM alpine/git:v2.54.1@sha256:2306e2b05ee0ef99f5d25b0779f645b24ae9f91ba187107503592fcdfe78da3a AS source

RUN git clone https://github.com/pwaller/pyfiglet.git /src \
    && cd /src \
    && git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf .git pyfiglet/fonts-contrib test-fonts \
    && mkdir -p pyfiglet/fonts \
    && cp pyfiglet/fonts-standard/* pyfiglet/fonts/

FROM golang:1.23.4-alpine3.20@sha256:6a84ccdb73e005d0ee7bfff6066f230612ca9dff3e88e31bfc752523c3a271f8

RUN apk add --no-cache bash \
    && adduser -D -u 1000 -h /home/player player
COPY --from=source --chown=1000:1000 /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo

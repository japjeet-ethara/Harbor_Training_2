# Stage 1: fetch pyfiglet and prepare the font files. This image is not the final image.
FROM alpine/git:v2.54.0@sha256:6a1fdb962814ca11508ca8d1bab94375b5b7ac4e4a9a40981c29fc32c02f095d AS source

RUN git clone https://github.com/pwaller/pyfiglet.git /src \
    && git -C /src checkout 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf /src/.git \
    && rm -rf /src/pyfiglet/fonts-contrib /src/test-fonts \
    && mkdir -p /src/pyfiglet/fonts \
    && cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

# Stage 2: the agent's room. Alpine Go has no Python; bash is the only extra package.
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f

RUN apk add --no-cache bash \
    && adduser -D -u 1000 -h /home/player player

COPY --from=source --chown=player:player /src /workspace/repo

ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

USER player
WORKDIR /workspace/repo

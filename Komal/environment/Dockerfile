FROM alpine/git:v2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS source
RUN git clone https://github.com/pwaller/pyfiglet /src \
    && git -C /src checkout --detach 255827efbc09d662875dad50fa0edc1e1263bf45 \
    && rm -rf /src/.git /src/pyfiglet/fonts-contrib /src/test-fonts \
    && mkdir /src/pyfiglet/fonts \
    && cp /src/pyfiglet/fonts-standard/* /src/pyfiglet/fonts/

FROM golang:1.23.12-alpine3.22@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f
RUN apk add --no-cache bash \
    && addgroup -g 1000 player \
    && adduser -D -u 1000 -G player player
COPY --from=source --chown=player:player /src /workspace/repo
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
USER player
WORKDIR /workspace/repo

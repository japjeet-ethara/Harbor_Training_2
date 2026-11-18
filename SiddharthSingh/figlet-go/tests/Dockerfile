FROM golang:1.23 AS go

FROM ubuntu:24.04

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOTOOLCHAIN=local

WORKDIR /app

COPY . /tests/

FROM alpine:edge as builder
LABEL stage=go-builder
WORKDIR /app/
RUN apk add --no-cache bash curl gcc git go musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY ./ ./
RUN bash build.sh release docker

FROM xiaoyaliu/alist:latest

LABEL MAINTAINER="Har01d"
RUN apk update && \
    apk upgrade --no-cache && \
    apk add --no-cache bash ca-certificates su-exec tzdata wget; \
    rm -rf /var/cache/apk/*

COPY --chmod=755 --from=builder /app/bin/alist ./

FROM alpine:edge as builder
LABEL stage=go-builder
WORKDIR /app/
COPY ./ ./
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN apk add --no-cache bash curl gcc git go musl-dev; \
    bash build.sh release docker

FROM xiaoyaliu/alist:latest

LABEL MAINTAINER="Har01d"

RUN apk add --no-cache wget

COPY --from=builder /app/bin/alist ./

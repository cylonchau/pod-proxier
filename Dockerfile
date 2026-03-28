FROM golang:alpine AS builder
MAINTAINER cylon
WORKDIR /build_root
COPY ./ /build_root
ENV GOPROXY https://goproxy.cn,direct
RUN \
    apk add upx  && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o pod-proxier-gateway main.go && \
    upx -1 pod-proxier-gateway && \
    chmod +x pod-proxier-gateway

FROM alpine AS runner
WORKDIR /apps
COPY --from=builder /build_root/pod-proxier-gateway .
VOLUME ["/apps"]
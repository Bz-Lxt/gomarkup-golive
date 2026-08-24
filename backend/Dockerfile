# syntax=docker/dockerfile:1

# -------- frontend --------
FROM node:22-alpine3.20 AS fe
WORKDIR /fe
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
COPY frontend-user/package.json frontend-user/package-lock.json* ./
RUN npm install --registry=https://registry.npmmirror.com
COPY frontend-user/ ./
RUN npm run build

# -------- backend --------
FROM golang:1.25-alpine3.22 AS be
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct \
    GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=fe /fe/dist/ ./cmd/server/static/
RUN go build -mod=mod -trimpath -ldflags="-s -w" -o /out/golive ./cmd/server

# -------- runtime --------
FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata wget \
 && addgroup -S golive && adduser -S -G golive golive \
 && mkdir -p /var/lib/golive/certs \
 && chown -R golive:golive /var/lib/golive
ENV TZ=Asia/Shanghai \
    QUIC_GO_DISABLE_GSO=1
WORKDIR /app
COPY --from=be /out/golive /app/golive
USER golive
EXPOSE 19443 19444/udp
ENTRYPOINT ["/app/golive"]

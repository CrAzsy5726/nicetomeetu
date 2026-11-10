# 构建阶段
FROM golang:1.26-alpine AS build
WORKDIR /app

# 国内 Go 模块代理（多个镜像兜底），避免默认 proxy.golang.org 连不上
ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct \
    GOSUMDB=off

# 先拷贝依赖文件，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码并编译为静态二进制
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

# 运行阶段
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /server /app/server

# 头像上传目录（后端启动时会自动创建）
ENV UPLOAD_DIR=/app/uploads
VOLUME /app/uploads

EXPOSE 8080
ENTRYPOINT ["/app/server"]

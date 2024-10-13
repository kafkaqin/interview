# 使用 Go 1.22.6 作为基础镜像进行编译
FROM golang:1.23-alpine3.19 AS builder

# 设置工作目录
WORKDIR /app

# 复制 Go 模块文件并下载依赖项
COPY go.mod go.sum ./
RUN go mod download

# 复制应用程序源代码
COPY . .

# 编译应用程序
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /scheduler-extender .

# 使用轻量级的运行时镜像
FROM alpine:3.19

# 安装 ca-certificates 以支持 HTTPS
# RUN apk --no-cache add ca-certificates

# 将编译后的应用程序复制到运行时镜像
COPY --from=builder /scheduler-extender /usr/local/bin/scheduler-extender

# 暴露服务端口（根据代码中设置的监听端口）
EXPOSE 8888

# 运行调度器扩展程序
ENTRYPOINT ["/usr/local/bin/scheduler-extender"]

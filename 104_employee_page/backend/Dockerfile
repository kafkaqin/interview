FROM golang:1.22-alpine3.19 AS builder

# 设置工作目录
WORKDIR /app

# 复制应用程序源代码
COPY . .

# 编译应用程序
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /employee-system-backend .

# 使用轻量级的运行时镜像
FROM alpine:3.19

# 安装 ca-certificates 以支持 HTTPS
# RUN apk --no-cache add ca-certificates

# 将编译后的应用程序复制到运行时镜像
COPY --from=builder /employee-system-backend /usr/local/bin/employee-system-backend

# 暴露服务端口（根据代码中设置的监听端口）
EXPOSE 8080

# 运行调度器扩展程序
ENTRYPOINT ["/usr/local/bin/employee-system-backend"]
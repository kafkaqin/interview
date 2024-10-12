# 使用官方的Golang镜像作为构建阶段
FROM golang:1.22.3 AS builder

# 设置工作目录
WORKDIR /app

# 将当前目录下的所有文件复制到工作目录
COPY . .

# 下载依赖并编译应用程序
RUN go mod tidy
RUN go build -o app .

# 使用一个更小的基础镜像来运行应用程序
FROM gcr.io/distroless/base-debian11

# 将编译好的二进制文件从构建阶段复制到运行阶段
COPY --from=builder /app/app /app

# 设置应用程序的入口点
ENTRYPOINT ["/app"]

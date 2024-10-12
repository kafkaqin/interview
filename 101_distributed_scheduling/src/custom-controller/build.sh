#!/bin/bash

# 构建 Docker 镜像
docker build -t scheduler-extender:1.0 .

# 运行容器
docker run -d -p 8888:8888 --name k8s-scheduler-extender scheduler-extender:1.0

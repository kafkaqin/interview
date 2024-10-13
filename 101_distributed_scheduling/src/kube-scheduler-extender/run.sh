#!/bin/bash

# 启动Docker容器
docker run --rm -v /path/to/kubeconfig:/path/to/kubeconfig:ro my-golang-app

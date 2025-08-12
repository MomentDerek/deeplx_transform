#!/bin/bash

# 构建并运行 DeepLX Transform 服务

echo "正在下载依赖..."
go mod download

echo "正在编译..."
go build -o deeplx_transform

if [ $? -eq 0 ]; then
    echo "编译成功，启动服务..."
    ./deeplx_transform
else
    echo "编译失败"
    exit 1
fi
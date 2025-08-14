# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

# 复制源代码
COPY main.go ./

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o deeplx_transform .

# 运行阶段
FROM alpine:latest

# 安装必要的工具
RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /app/deeplx_transform .

# 复制配置文件
COPY config.yaml .

# 暴露端口
EXPOSE 8080

# 运行程序
CMD ["./deeplx_transform"]
# 第一阶段：构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 设置 Go 代理（加快国内构建速度）
ENV GOPROXY=https://goproxy.cn,direct

# 复制依赖文件并下载
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译项目
# -ldflags="-s -w" 用于减小二进制体积
# -o personal_assistant 指定输出文件名
RUN go build -ldflags="-s -w" -o personal_assistant cmd/main.go

# 第二阶段：运行阶段
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 安装基础依赖（如时区设置）
RUN apk add --no-cache tzdata

# 从构建阶段复制二进制文件
COPY --from=builder /app/personal_assistant .

# 复制必要的配置文件
# 注意：configs.yaml 包含默认配置，生产环境推荐用环境变量覆盖
COPY configs/configs.yaml configs/configs.yaml
COPY configs/model.conf configs/model.conf

# 创建挂载点目录（防止目录不存在报错）
RUN mkdir -p static/images log

# 暴露端口
EXPOSE 8002

# 启动命令
CMD ["./personal_assistant"]

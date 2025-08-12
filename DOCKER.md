# Docker 部署指南

## 快速开始

### 1. 基本使用

```bash
# 克隆项目
git clone <repository-url>
cd deeplx_transform

# 使用 docker-compose 启动
docker-compose up -d

# 查看运行状态
docker-compose ps

# 查看日志
docker-compose logs -f
```

服务将在 `http://localhost:8080` 上运行。

### 2. 配置说明

#### 使用配置文件
编辑 `config.yaml` 文件：
```yaml
target:
  base_url: "https://api.deeplx.org"  # 修改为你的目标服务地址
```

#### 使用环境变量
创建 `.env` 文件（可以从 `.env.example` 复制）：
```bash
cp .env.example .env
# 编辑 .env 文件
```

环境变量说明：
- `HOST_PORT`: 主机端口（默认 8080）
- `CONTAINER_PORT`: 容器内部端口（默认 8080）
- `GIN_MODE`: Gin 框架模式（debug/release/test）
- `TARGET_BASE_URL`: 目标服务基础 URL
- `CONTAINER_NAME`: 容器名称
- `RESTART_POLICY`: 重启策略

## 常用命令

### 服务管理

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose stop

# 重启服务
docker-compose restart

# 删除服务（保留镜像）
docker-compose down

# 删除服务和镜像
docker-compose down --rmi all

# 查看服务状态
docker-compose ps

# 查看服务健康状态
docker inspect deeplx-transform --format='{{.State.Health.Status}}'
```

### 日志管理

```bash
# 查看所有日志
docker-compose logs

# 实时查看日志
docker-compose logs -f

# 查看最近 N 行日志
docker-compose logs --tail=100

# 查看特定时间范围的日志
docker-compose logs --since="2024-01-01" --until="2024-01-02"
```

### 更新和重建

```bash
# 拉取最新代码后重建
git pull
docker-compose build
docker-compose up -d

# 一步完成重建和启动
docker-compose up -d --build

# 强制重建（不使用缓存）
docker-compose build --no-cache
docker-compose up -d
```

## 高级配置

### 1. 自定义网络

创建 `docker-compose.override.yml`:
```yaml
version: '3.8'

services:
  deeplx-transform:
    networks:
      - custom-network

networks:
  custom-network:
    driver: bridge
```

### 2. 资源限制

在 `docker-compose.yml` 中添加：
```yaml
services:
  deeplx-transform:
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

### 3. 持久化日志

修改 `docker-compose.yml`:
```yaml
services:
  deeplx-transform:
    volumes:
      - ./config.yaml:/root/config.yaml
      - ./logs:/root/logs  # 添加日志目录映射
```

### 4. 使用外部配置

```bash
# 使用不同的配置文件
docker run -v /path/to/custom-config.yaml:/root/config.yaml deeplx-transform
```

## 生产环境部署

### 1. 使用 Docker Swarm

```bash
# 初始化 Swarm
docker swarm init

# 部署服务
docker stack deploy -c docker-compose.yml deeplx-stack

# 查看服务
docker service ls
docker service logs deeplx-stack_deeplx-transform
```

### 2. 使用 Kubernetes

创建 `kubernetes.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deeplx-transform
spec:
  replicas: 3
  selector:
    matchLabels:
      app: deeplx-transform
  template:
    metadata:
      labels:
        app: deeplx-transform
    spec:
      containers:
      - name: deeplx-transform
        image: deeplx-transform:latest
        ports:
        - containerPort: 8080
        env:
        - name: GIN_MODE
          value: "release"
        - name: TARGET_BASE_URL
          value: "https://api.deeplx.org"
---
apiVersion: v1
kind: Service
metadata:
  name: deeplx-transform-service
spec:
  selector:
    app: deeplx-transform
  ports:
    - port: 80
      targetPort: 8080
  type: LoadBalancer
```

部署到 Kubernetes:
```bash
kubectl apply -f kubernetes.yaml
kubectl get pods
kubectl get services
```

## 故障排查

### 1. 容器无法启动

```bash
# 查看详细错误
docker-compose logs
docker-compose ps

# 检查配置文件
docker-compose config

# 验证 Dockerfile
docker build -t test .
```

### 2. 健康检查失败

```bash
# 进入容器内部测试
docker exec -it deeplx-transform sh
wget -O- http://localhost:8080/health

# 检查网络
docker network ls
docker network inspect <network-name>
```

### 3. 性能问题

```bash
# 查看资源使用
docker stats deeplx-transform

# 查看容器详情
docker inspect deeplx-transform

# 查看日志中的错误
docker-compose logs | grep ERROR
```

## 安全建议

1. **不要在生产环境暴露调试信息**
   ```yaml
   debug:
     enabled: false  # 生产环境设置为 false
   ```

2. **使用 HTTPS**
   - 在反向代理（如 Nginx）后面部署
   - 使用 Let's Encrypt 证书

3. **限制网络访问**
   ```yaml
   ports:
     - "127.0.0.1:8080:8080"  # 只监听本地
   ```

4. **定期更新基础镜像**
   ```bash
   docker pull alpine:latest
   docker-compose build --no-cache
   ```

## 监控和日志

### 使用 Prometheus + Grafana

1. 添加 metrics 端点到应用
2. 配置 Prometheus 采集
3. 在 Grafana 中创建仪表板

### 使用 ELK Stack

1. 配置日志输出为 JSON 格式
2. 使用 Filebeat 收集日志
3. 发送到 Elasticsearch
4. 在 Kibana 中查看

## 备份和恢复

```bash
# 备份配置
tar -czf backup.tar.gz config.yaml docker-compose.yml

# 导出镜像
docker save deeplx-transform:latest > deeplx-transform.tar

# 恢复镜像
docker load < deeplx-transform.tar
```
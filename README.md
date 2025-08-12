# DeepLX Transform - V2 到 V1 API 转发器

这个服务将 DeepL V2 格式的翻译请求转换为 V1 格式，并将响应转换回 V2 格式。

## 主要特性

- 接收 `/v2/translate` 格式的请求
- 将授权 token 从请求头提取并放入 URL 路径中
- 转发到目标服务的 `/{token}/translate` 端点
- 自动转换请求和响应格式
- **支持并发翻译多个文本**（V2 API 的 text 数组）
- 可配置的并发限制和超时控制

## 配置

编辑 `config.yaml` 文件：

```yaml
server:
  port: "8080"  # 服务监听端口

target:
  base_url: "http://localhost:3000"  # 目标服务的基础 URL
  default_source_lang: "auto"  # 默认源语言

performance:
  max_concurrent_requests: 10  # 最大并发请求数
  request_timeout: 30  # 单个请求超时时间（秒）

debug:
  enabled: true  # 是否启用调试日志
  log_request_body: true  # 记录请求体
  log_response_body: true  # 记录响应体
  log_headers: true  # 记录请求头
```

## 运行

```bash
# 安装依赖
go mod download

# 运行服务
go run main.go

# 或者编译后运行
go build -o deeplx_transform
./deeplx_transform
```

## API 使用

### 请求示例

单个文本：
```bash
curl -X POST http://localhost:8080/v2/translate \
  -H "Content-Type: application/json" \
  -H "Authorization: DeepL-Auth-Key your_access_token" \
  -d '{
    "text": ["Hello world"],
    "target_lang": "ZH"
  }'
```

多个文本（并发翻译）：
```bash
curl -X POST http://localhost:8080/v2/translate \
  -H "Content-Type: application/json" \
  -H "Authorization: DeepL-Auth-Key your_access_token" \
  -d '{
    "text": [
      "Hello world",
      "How are you?",
      "Good morning"
    ],
    "target_lang": "ZH"
  }'
```

### 响应示例

单个文本响应：
```json
{
  "translations": [
    {
      "detected_source_language": "EN",
      "text": "你好世界"
    }
  ]
}
```

多个文本响应：
```json
{
  "translations": [
    {
      "detected_source_language": "EN",
      "text": "你好世界"
    },
    {
      "detected_source_language": "EN",
      "text": "你好吗？"
    },
    {
      "detected_source_language": "EN",
      "text": "早上好"
    }
  ]
}
```

## 鉴权流程

1. 客户端发送带有 `Authorization: DeepL-Auth-Key [token]` 头的请求到 `/v2/translate`
2. 转发器提取 token
3. 构建目标 URL: `{base_url}/{token}/translate`
4. 转发请求到目标服务
5. 将 V1 格式响应转换为 V2 格式返回

## 并发翻译特性

转发器支持并发处理 V2 API 的文本数组：

- **并发执行**：同时发送多个翻译请求到上游服务
- **并发控制**：通过 `max_concurrent_requests` 限制最大并发数
- **超时保护**：每个请求都有独立的超时控制
- **顺序保持**：响应结果按照输入顺序排列
- **部分失败处理**：如果某些文本翻译失败，其他成功的翻译仍会返回

## 调试功能

启用调试模式后，服务会输出详细的日志信息，包括：

- 每个请求的唯一 ID（基于时间戳）
- 子请求 ID（格式：主ID-索引）用于追踪并发任务
- 原始请求头和请求体
- 授权 token 提取过程
- V2 到 V1 的请求转换
- 并发任务的启动和完成状态
- 每个并发请求的响应时间
- 总体处理时间统计
- 原始响应体
- V1 到 V2 的响应转换

调试日志示例：
```
[DEBUG] [1702345678901234567] 收到 /v2/translate 请求
[DEBUG] [1702345678901234567] 准备并发翻译 3 个文本
[DEBUG] [1702345678901234567-0] 开始翻译文本[0]
[DEBUG] [1702345678901234567-1] 开始翻译文本[1]
[DEBUG] [1702345678901234567-2] 开始翻译文本[2]
[DEBUG] [1702345678901234567-1] 响应状态: 200 (耗时: 120ms)
[DEBUG] [1702345678901234567-0] 响应状态: 200 (耗时: 145ms)
[DEBUG] [1702345678901234567-2] 响应状态: 200 (耗时: 189ms)
[DEBUG] [1702345678901234567] 所有翻译完成，总耗时: 189ms
```

## 环境变量

可以通过环境变量覆盖配置：

- `TARGET_BASE_URL`: 目标服务的基础 URL
- `GIN_MODE`: Gin 框架模式 (debug/release/test)
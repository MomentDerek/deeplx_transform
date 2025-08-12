package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

// 配置结构
type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Target struct {
		BaseURL             string `yaml:"base_url"`
		DefaultSourceLang string `yaml:"default_source_lang"`
	} `yaml:"target"`
	Performance struct {
		MaxConcurrentRequests int `yaml:"max_concurrent_requests"`
		RequestTimeout        int `yaml:"request_timeout"`
	} `yaml:"performance"`
	Debug struct {
		Enabled         bool `yaml:"enabled"`
		LogRequestBody  bool `yaml:"log_request_body"`
		LogResponseBody bool `yaml:"log_response_body"`
		LogHeaders      bool `yaml:"log_headers"`
	} `yaml:"debug"`
}

// V2 请求结构
type V2TranslateRequest struct {
	Text       []string `json:"text"`
	SourceLang string   `json:"source_lang,omitempty"`
	TargetLang string   `json:"target_lang"`
}

// V1 请求结构
type V1TranslateRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

// V1 响应结构
type V1TranslateResponse struct {
	Alternatives []string `json:"alternatives"`
	Code         int      `json:"code"`
	Data         string   `json:"data"`
	ID           int64    `json:"id"`
	Method       string   `json:"method"`
	SourceLang   string   `json:"source_lang"`
	TargetLang   string   `json:"target_lang"`
}

// V2 响应结构
type V2TranslateResponse struct {
	Translations []Translation `json:"translations"`
}

type Translation struct {
	DetectedSourceLanguage string `json:"detected_source_language"`
	Text                   string `json:"text"`
}

// 翻译任务结构
type translateTask struct {
	index      int
	text       string
	result     string
	sourceLang string
	error      error
}

// 翻译结果结构
type translateResult struct {
	index      int
	translation Translation
	error      error
}

var config Config

// 调试日志函数
func debugLog(format string, args ...interface{}) {
	if config.Debug.Enabled {
		log.Printf("[DEBUG] " + format, args...)
	}
}

// 将请求/响应体格式化为可读的 JSON
func formatJSON(data []byte) string {
	var result bytes.Buffer
	if err := json.Indent(&result, data, "", "  "); err != nil {
		return string(data)
	}
	return result.String()
}

// 加载配置文件
func loadConfig() error {
	// 首先尝试从环境变量获取配置
	if targetBaseURL := os.Getenv("TARGET_BASE_URL"); targetBaseURL != "" {
		config.Target.BaseURL = targetBaseURL
	} else {
		// 从配置文件加载
		file, err := os.Open("config.yaml")
		if err != nil {
			// 使用默认配置
			config.Server.Port = "8080"
			config.Target.BaseURL = "http://localhost:3000"
			config.Target.DefaultSourceLang = "auto"
			log.Println("使用默认配置")
			return nil
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return fmt.Errorf("解析配置文件失败: %v", err)
		}
	}

	// 设置默认值
	if config.Server.Port == "" {
		config.Server.Port = "8080"
	}
	if config.Target.DefaultSourceLang == "" {
		config.Target.DefaultSourceLang = "auto"
	}
	if config.Performance.MaxConcurrentRequests <= 0 {
		config.Performance.MaxConcurrentRequests = 10
	}
	if config.Performance.RequestTimeout <= 0 {
		config.Performance.RequestTimeout = 30
	}

	return nil
}

// 从授权头提取 token
func extractToken(v2Auth string) string {
	// v2格式: "DeepL-Auth-Key [yourAccessToken]"
	// 提取 token 用于构建 URL 路径
	
	if strings.HasPrefix(v2Auth, "DeepL-Auth-Key ") {
		return strings.TrimPrefix(v2Auth, "DeepL-Auth-Key ")
	}
	
	// 如果是 Bearer 格式，也提取 token
	if strings.HasPrefix(v2Auth, "Bearer ") {
		return strings.TrimPrefix(v2Auth, "Bearer ")
	}
	
	// 如果没有前缀，假设整个字符串就是 token
	return v2Auth
}

// 处理 /v2/translate 请求
func handleV2Translate(c *gin.Context) {
	// 记录请求开始
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())
	debugLog("[%s] 收到 /v2/translate 请求", requestID)
	
	// 记录请求头
	if config.Debug.LogHeaders {
		debugLog("[%s] 请求头:", requestID)
		for key, values := range c.Request.Header {
			for _, value := range values {
				debugLog("  %s: %s", key, value)
			}
		}
	}
	
	// 解析 v2 请求
	var v2Req V2TranslateRequest
	if err := c.ShouldBindJSON(&v2Req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式",
			"details": err.Error(),
		})
		return
	}

	// 记录请求体
	if config.Debug.LogRequestBody {
		reqBodyJSON, _ := json.Marshal(v2Req)
		debugLog("[%s] V2 请求体:\n%s", requestID, formatJSON(reqBodyJSON))
	}
	
	// 验证必需参数
	if len(v2Req.Text) == 0 {
		debugLog("[%s] 请求验证失败: text 数组为空", requestID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "text 参数是必需的",
		})
		return
	}
	
	// 检查是否有空文本
	for i, text := range v2Req.Text {
		if text == "" {
			debugLog("[%s] 请求验证失败: text[%d] 为空", requestID, i)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("text[%d] 不能为空", i),
			})
			return
		}
	}
	if v2Req.TargetLang == "" {
		debugLog("[%s] 请求验证失败: target_lang 参数为空", requestID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "target_lang 参数是必需的",
		})
		return
	}

	// 获取授权头并提取 token
	v2AuthHeader := c.GetHeader("Authorization")
	debugLog("[%s] 原始授权头: %s", requestID, v2AuthHeader)
	
	token := extractToken(v2AuthHeader)
	debugLog("[%s] 提取的 token: %s", requestID, token)
	
	// 如果没有 token，返回未授权错误
	if token == "" {
		debugLog("[%s] 授权失败: 未找到 token", requestID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "缺少授权信息",
			"details": "请在 Authorization 头中提供 'DeepL-Auth-Key [token]' 格式的授权信息",
		})
		return
	}

	// 为 v1 API 准备源语言（v1 中必需）
	sourceLang := v2Req.SourceLang
	if sourceLang == "" {
		sourceLang = config.Target.DefaultSourceLang
		debugLog("[%s] 使用默认源语言: %s", requestID, sourceLang)
	} else {
		debugLog("[%s] 使用指定源语言: %s", requestID, sourceLang)
	}

	// 构建目标 URL
	targetURL := fmt.Sprintf("%s/%s/translate", config.Target.BaseURL, token)
	debugLog("[%s] 目标 URL: %s", requestID, targetURL)
	debugLog("[%s] 准备并发翻译 %d 个文本", requestID, len(v2Req.Text))

	// 并发翻译所有文本
	results := make([]translateResult, len(v2Req.Text))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, config.Performance.MaxConcurrentRequests)
	
	startTime := time.Now()
	
	for i, text := range v2Req.Text {
		wg.Add(1)
		go func(index int, textToTranslate string) {
			defer wg.Done()
			
			// 使用信号量控制并发数
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			subRequestID := fmt.Sprintf("%s-%d", requestID, index)
			debugLog("[%s] 开始翻译文本[%d]", subRequestID, index)
			
			// 调用单个文本翻译函数
			translation, err := translateSingleText(
				subRequestID,
				textToTranslate,
				sourceLang,
				v2Req.TargetLang,
				targetURL,
			)
			
			if err != nil {
				debugLog("[%s] 翻译失败: %v", subRequestID, err)
				results[index] = translateResult{
					index: index,
					error: err,
				}
			} else {
				debugLog("[%s] 翻译成功", subRequestID)
				results[index] = translateResult{
					index:       index,
					translation: *translation,
					error:       nil,
				}
			}
		}(i, text)
	}
	
	// 等待所有翻译完成
	wg.Wait()
	elapsed := time.Since(startTime)
	debugLog("[%s] 所有翻译完成，总耗时: %v", requestID, elapsed)
	
	// 构建 V2 响应
	v2Resp := V2TranslateResponse{
		Translations: make([]Translation, 0, len(results)),
	}
	
	// 按照原始顺序组装结果
	allSuccess := true
	for _, result := range results {
		if result.error != nil {
			allSuccess = false
			// 如果某个翻译失败，返回空翻译或错误标记
			v2Resp.Translations = append(v2Resp.Translations, Translation{
				DetectedSourceLanguage: sourceLang,
				Text:                   fmt.Sprintf("[TRANSLATION ERROR: %v]", result.error),
			})
		} else {
			v2Resp.Translations = append(v2Resp.Translations, result.translation)
		}
	}
	
	// 记录转换后的 V2 响应
	if config.Debug.LogResponseBody {
		v2RespJSON, _ := json.Marshal(v2Resp)
		debugLog("[%s] 转换后的 V2 响应:\n%s", requestID, formatJSON(v2RespJSON))
	}
	
	// 返回结果
	if !allSuccess {
		debugLog("[%s] 部分翻译失败，返回部分结果", requestID)
		c.JSON(http.StatusPartialContent, v2Resp)
	} else {
		debugLog("[%s] 所有翻译成功，返回完整结果", requestID)
		c.JSON(http.StatusOK, v2Resp)
	}
}

// 翻译单个文本
func translateSingleText(requestID, text, sourceLang, targetLang, targetURL string) (*Translation, error) {
	// 创建 V1 请求
	v1Req := V1TranslateRequest{
		Text:       text,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}
	
	// 记录请求
	if config.Debug.LogRequestBody {
		v1ReqJSON, _ := json.Marshal(v1Req)
		debugLog("[%s] V1 请求体:\n%s", requestID, formatJSON(v1ReqJSON))
	}
	
	// 序列化请求体
	reqBody, err := json.Marshal(v1Req)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %v", err)
	}
	
	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	
	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: time.Duration(config.Performance.RequestTimeout) * time.Second,
	}
	
	// 发送请求
	startTime := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(startTime)
	
	if err != nil {
		return nil, fmt.Errorf("请求失败 (耗时: %v): %v", elapsed, err)
	}
	defer resp.Body.Close()
	
	debugLog("[%s] 响应状态: %d (耗时: %v)", requestID, resp.StatusCode, elapsed)
	
	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	
	// 记录响应
	if config.Debug.LogResponseBody {
		debugLog("[%s] V1 响应:\n%s", requestID, formatJSON(body))
	}
	
	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("上游返回错误: %d - %s", resp.StatusCode, string(body))
	}
	
	// 解析 V1 响应
	var v1Resp V1TranslateResponse
	if err := json.Unmarshal(body, &v1Resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	
	// 返回转换后的结果
	return &Translation{
		DetectedSourceLanguage: v1Resp.SourceLang,
		Text:                   v1Resp.Data,
	}, nil
}

// 健康检查端点
func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "v2-to-v1-translator",
	})
}

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("转发器配置:")
	log.Printf("  监听端口: %s", config.Server.Port)
	log.Printf("  目标基础 URL: %s", config.Target.BaseURL)
	log.Printf("  默认源语言: %s", config.Target.DefaultSourceLang)
	log.Printf("  最大并发请求数: %d", config.Performance.MaxConcurrentRequests)
	log.Printf("  请求超时时间: %d秒", config.Performance.RequestTimeout)
	log.Printf("  调试模式: %v", config.Debug.Enabled)
	if config.Debug.Enabled {
		log.Printf("  - 记录请求体: %v", config.Debug.LogRequestBody)
		log.Printf("  - 记录响应体: %v", config.Debug.LogResponseBody)
		log.Printf("  - 记录请求头: %v", config.Debug.LogHeaders)
	}

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	r := gin.Default()

	// 添加路由
	r.POST("/v2/translate", handleV2Translate)
	r.GET("/health", handleHealth)

	// 启动服务器
	addr := ":" + config.Server.Port
	log.Printf("转发器启动在 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
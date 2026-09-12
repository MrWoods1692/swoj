package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAISystemPrompt 是编程场景的默认模型提示词；管理员可在后台覆盖。
const DefaultAISystemPrompt = "你是一名算法竞赛编程助手。你的服务对象是做算法题的程序员与学生，" +
	"熟悉 C++ 标准库与常见竞赛套路。回答时优先给出可直接执行的思路与代码片段，" +
	"能读懂 .cpp / .txt / .in / .out 等文件内容；指出错误时说明位置、原因与修复方式；" +
	"遇到 TLE/RE/WA 时给出复杂度分析或边界建议。回答使用中文，简洁直白。"

// callAI 根据 Provider 分派到具体实现；返回 (answer, source, error)。
func callAI(cfg *AIConfig, prompt string) (string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "yunzhi":
		return callAIYunzhi(cfg, prompt)
	case "", "openai":
		return callAIOpenAI(cfg, prompt)
	default:
		return "", "", fmt.Errorf("未知 AI Provider: %s", provider)
	}
}

// callAIOpenAI 调用 OpenAI 兼容的 chat/completions 接口。
func callAIOpenAI(cfg *AIConfig, prompt string) (string, string, error) {
	if cfg.APIKey == "" {
		return "", "", fmt.Errorf("未配置 AI API Key")
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	systemPrompt := effectiveSystemPrompt(cfg)

	body, err := json.Marshal(map[string]any{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": cfg.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("编码请求体失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL, strings.NewReader(string(body)))
	if err != nil {
		return "", "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求 AI 服务失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode, truncateStr(string(raw), 400))
	}
	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", "", fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", "", fmt.Errorf("AI 服务未返回内容")
	}
	if parsed.Model == "" {
		parsed.Model = model
	}
	return parsed.Choices[0].Message.Content, parsed.Model, nil
}

// callAIYunzhi 调用 yunzhiapi.cn 的 DeepSeek 代理。
// GET https://yunzhiapi.cn/API/deepseek.php?question=&system=&token=
//
// token 由管理员在后台配置，存 admin_configs('ai.token')；此处从 cfg.YunzhiToken 读取。
// 官方文档：200 成功、500 请求超时。
func callAIYunzhi(cfg *AIConfig, prompt string) (string, string, error) {
	if cfg.YunzhiToken == "" {
		return "", "", fmt.Errorf("未配置 AI Token（管理员后台 → AI 配置）")
	}
	apiURL := strings.TrimSpace(cfg.YunzhiURL)
	if apiURL == "" {
		apiURL = "https://yunzhiapi.cn/API/deepseek.php"
	}
	systemPrompt := effectiveSystemPrompt(cfg)

	// 提问与提示词都走 query string；type=text 保证纯文本返回。
	q := url.Values{}
	q.Set("question", prompt)
	if systemPrompt != "" {
		q.Set("system", systemPrompt)
	}
	q.Set("type", "text")
	q.Set("token", cfg.YunzhiToken)
	sep := "?"
	if strings.Contains(apiURL, "?") {
		sep = "&"
	}
	fullURL := apiURL + sep + q.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// 显式使用 GET 方法访问远端 AI 代理。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Accept", "text/plain, */*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求 AI 服务失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	text := strings.TrimSpace(string(raw))
	if resp.StatusCode == http.StatusInternalServerError {
		// AI 服务超时
		return "", "", fmt.Errorf("AI 服务超时")
	}
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode, truncateStr(string(raw), 400))
	}
	if text == "" {
		return "", "", fmt.Errorf("AI 服务未返回内容")
	}
	return text, "deepseek-r1", nil
}

// effectiveSystemPrompt 返回用于发送给模型的系统提示词。
// 优先级：管理员覆盖的 SystemPrompt > 默认 DefaultAISystemPrompt。
func effectiveSystemPrompt(cfg *AIConfig) string {
	if p := strings.TrimSpace(cfg.SystemPrompt); p != "" {
		return p
	}
	return DefaultAISystemPrompt
}

// truncateStr 截断字符串，避免错误消息过长。
func truncateStr(s string, max int) string {
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

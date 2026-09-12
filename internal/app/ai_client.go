package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// callAI 调用 OpenAI 兼容的 chat/completions 接口，返回回答文本与模型名。
func callAI(cfg *AIConfig, prompt string) (string, string, error) {
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

	body, err := json.Marshal(map[string]any{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": cfg.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": "你是算法竞赛辅导助手，回答使用中文。"},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("编码请求体失败: %w", err)
	}

	ctx, cancel := withTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL, bytes.NewReader(body))
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
		return "", "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode, string(raw))
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

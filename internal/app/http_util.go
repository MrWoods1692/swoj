package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CtxKey 请求上下文键。
type CtxKey string

const (
	CtxClaims CtxKey = "claims"
	CtxIP     CtxKey = "ip"
)

// APIResponse 统一 API 响应结构。
type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// OK 返回成功响应。
func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, APIResponse{Code: 200, Msg: "ok", Data: data})
}

// Fail 返回失败响应。
func Fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, APIResponse{Code: status, Msg: msg})
}

// Failf 返回带格式化消息的失败响应。
func Failf(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, APIResponse{Code: status, Msg: fmt.Sprintf(format, args...)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode 解析 JSON 请求体，并限制大小防止请求体过大。
// 无请求体（空 body）时视为零值结构，便于 GET 带查询参数的调用复用。
func decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return fmt.Errorf("请求参数格式错误: %w", err)
	}
	return nil
}

// clientIP 取请求方 IP，优先解析 X-Forwarded-For 首段。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		return host[:i]
	}
	return host
}

// ErrBlocked IP 被禁用。
var ErrBlocked = errors.New("该 IP 已被禁用")

// ClaimsFrom 从上下文取出鉴权后的用户声明。
func ClaimsFrom(r *http.Request) (*Claims, bool) {
	c, ok := r.Context().Value(CtxClaims).(*Claims)
	return c, ok
}

// AuthContext 将用户声明注入请求上下文。
func AuthContext(r *http.Request, c *Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), CtxClaims, c))
}

// RequireRole 断言当前用户角色。
func RequireRole(c *Claims, roles ...string) bool {
	for _, r := range roles {
		if c.Role == r {
			return true
		}
	}
	return false
}

// logOp 记录操作日志。
func logOp(db *DB, c *Claims, action, target, detail, ip string) {
	uid, uname := int64(0), ""
	if c != nil {
		uid, uname = c.UserID, ""
	}
	_, _ = db.Exec(`INSERT INTO operation_logs(user_id,username,action,target,detail,ip)
		VALUES(?,?,?,?,?,?)`, uid, uname, action, target, detail, ip)
}

// withTimeout 为下游调用设置超时。
func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// withIP 注入客户端 IP。
func withIP(r *http.Request, ip string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), CtxIP, ip))
}

// withValue 注入上下文值。
func withValue(r *http.Request, k CtxKey, v any) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), k, v))
}

// IssueCSRF 签发 CSRF Cookie 并返回令牌，供前端在变更类请求中回传。
func IssueCSRF(w http.ResponseWriter) string {
	tok := hexID()
	http.SetCookie(w, &http.Cookie{
		Name: "swoj_csrf", Value: tok,
		Path: "/", MaxAge: 86400, HttpOnly: false, SameSite: http.SameSiteLaxMode,
	})
	return tok
}

// hexID 生成短随机十六进制标识，用于 CSRF 令牌、OAuth state 等一次性凭据。
func hexID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

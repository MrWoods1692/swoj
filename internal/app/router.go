package app

import (
	"net/http"
	"strings"
)

// HandlerFunc 处理器签名，便于组合中间件。
type HandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP 适配 http.Handler。
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) { h(w, r) }

// use 串联中间件。
func use(h HandlerFunc, mids ...func(HandlerFunc) HandlerFunc) HandlerFunc {
	for i := len(mids) - 1; i >= 0; i-- {
		h = mids[i](h)
	}
	return h
}

// authMiddleware 校验 JWT 并把用户声明注入上下文。
type authMiddleware struct{ s *Server }

func (m authMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := m.token(r)
		if err != nil || token == "" {
			Fail(w, http.StatusUnauthorized, "请先登录")
			return
		}
		claims, err := ParseToken(m.s.cfg.JWTSecret, token)
		if err != nil {
			Fail(w, http.StatusUnauthorized, err.Error())
			return
		}
		next(w, AuthContext(r, claims))
	}
}

// token 从 Authorization 头或 Cookie 读取令牌。
func (m authMiddleware) token(r *http.Request) (string, error) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):]), nil
	}
	c, err := r.Cookie("swoj_token")
	if err != nil {
		return "", nil
	}
	return c.Value, nil
}

// csrfMiddleware 对变更类请求校验 CSRF。
func csrfMiddleware() func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead ||
				r.Method == http.MethodOptions {
				next(w, r)
				return
			}
			cookie, err := r.Cookie("swoj_csrf")
			if err != nil || cookie.Value != r.Header.Get("X-CSRF-Token") {
				Fail(w, http.StatusForbidden, "CSRF 校验失败")
				return
			}
			next(w, r)
		}
	}
}

// ipBlockMiddleware 拒绝被禁用的来源 IP。
func ipBlockMiddleware(db *DB) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			var n int
			_ = db.QueryRow(`SELECT COUNT(*) FROM ip_blocks WHERE ip=?`, ip).Scan(&n)
			if n > 0 {
				Fail(w, http.StatusForbidden, ErrBlocked.Error())
				return
			}
			next(w, r)
		}
	}
}

// recoveryMiddleware 捕获 panic，返回 500 而非断开连接。
func recoveryMiddleware(next HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				Fail(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next(w, r)
	}
}

// corsMiddleware 允许开发期跨源访问。
func corsMiddleware(next HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// NewRouter 注册全部 API 路由，返回根 mux。
func NewRouter(s *Server) http.Handler {
	mux := http.NewServeMux()
	auth := authMiddleware{s}
	base := []func(HandlerFunc) HandlerFunc{
		recoveryMiddleware, corsMiddleware, ipBlockMiddleware(s.db), csrfMiddleware(),
	}

	// add 注册带中间件链的处理器。
	add := func(pattern string, h http.Handler, mid ...func(HandlerFunc) HandlerFunc) {
		fn := use(HandlerFunc(h.ServeHTTP), append(base, mid...)...)
		mux.Handle(pattern, fn)
	}
	// pub 公开路由。
	pub := func(pattern string, h HandlerFunc) { add(pattern, http.HandlerFunc(h)) }
	// pri 需要登录的路由。
	pri := func(pattern string, h HandlerFunc) { add(pattern, http.HandlerFunc(h), auth.Wrap) }

	pub("GET /api/csrf", func(w http.ResponseWriter, r *http.Request) {
		OK(w, map[string]any{"csrf": IssueCSRF(w)})
	})
	pub("POST /api/auth/login", s.login)
	pub("POST /api/auth/register", s.register)
	pub("POST /api/auth/logout", s.logout)
	pub("GET /api/auth/me", s.me)
	pub("GET /api/problems", s.problemList)
	pub("GET /api/problems/{id}", s.problemDetail)
	pri("POST /api/submissions", s.submit)
	return mux
}

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
	// 密码登录已全部下线：登录只走校园墙授权，首位完成授权的用户自动成为管理员。
	pub("POST /api/auth/logout", s.logout)
	pub("GET /auth/campux", s.oauthAuthorize)
	pub("GET "+oauthCallbackPath, s.oauthCallback)
	// 登录态接口统一走鉴权中间件，避免公开路由误开。
	pri("GET /api/auth/me", s.me)
	pri("PUT /api/auth/me", s.profileUpdate)
	// 公开个人主页：任何访客可查看，前端外链 /profile/{id} 直接渲染此数据。
	pub("GET /api/users/{id}/homepage", s.userHomepage)
	pub("GET /api/problems", s.problemList)
	pub("GET /api/problems/{id}", s.problemDetail)
	pri("POST /api/submissions", s.submit)

	// 提交记录
	pri("GET /api/submissions", s.submissionList)
	pri("GET /api/submissions/{id}", s.submissionDetail)

	// 比赛
	pub("GET /api/contests", s.contestList)
	pri("POST /api/contests", s.contestCreate)
	pub("GET /api/contests/{id}", s.contestDetail)
	pub("GET /api/contests/{id}/rank", s.contestRank)
	pri("POST /api/contests/{id}/enroll", s.contestEnroll)

	// 作业
	pub("GET /api/assignments", s.assignmentList)
	pub("GET /api/assignments/{id}", s.assignmentDetail)
	pri("POST /api/assignments", s.assignmentCreate)

	// 训练计划
	pub("GET /api/training-plans", s.planList)
	pub("GET /api/training-plans/{id}", s.planDetail)
	pri("POST /api/training-plans", s.planCreate)

	// 讨论
	pub("GET /api/discussions", s.discussionList)
	pub("GET /api/discussions/{id}", s.discussionDetail)
	pri("POST /api/discussions", s.discussionCreate)
	pri("POST /api/discussions/{id}/reply", s.discussionReply)
	pri("POST /api/discussions/{id}/like", s.discussionLike)

	// 排行榜与个人统计
	pub("GET /api/leaderboard", s.leaderboard)
	pri("GET /api/auth/me/stats", s.myStats)

	// 错题本
	pri("GET /api/wrong-questions", s.wrongQuestionList)
	pri("DELETE /api/wrong-questions/{id}", s.wrongQuestionRemove)

	// 服务状态（公开）与服务端健康检查
	pub("GET /api/status", s.serviceStatus)

	// 管理后台：题目管理
	pri("POST /api/admin/problems", s.problemCreate)
	pri("POST /api/admin/problems/{id}", s.problemUpdate)
	pri("DELETE /api/admin/problems/{id}", s.problemDelete)

	// 管理后台：测评节点与队列监控
	pri("GET /api/admin/nodes", s.nodeList)
	pri("POST /api/admin/nodes", s.nodeCreate)
	pri("PUT /api/admin/nodes/{id}", s.nodeUpdate)
	pri("DELETE /api/admin/nodes/{id}", s.nodeDelete)
	pri("GET /api/admin/queue", s.queueStats)

	// 管理后台：用户管理
	pri("GET /api/admin/users", s.userList)
	pri("PUT /api/admin/users/{id}", s.userUpdate)

	// 管理后台：IP 封禁
	pri("GET /api/admin/ip-blocks", s.ipBlockList)
	pri("POST /api/admin/ip-blocks", s.ipBlockAdd)
	pri("DELETE /api/admin/ip-blocks/{id}", s.ipBlockRemove)

	// 管理后台：系统配置与操作日志
	pri("GET /api/admin/config", s.configList)
	pri("POST /api/admin/config", s.configSet)
	pri("GET /api/admin/logs", s.opLogs)

	// 积分系统
	pub("GET /api/points/rules", s.pointsRulesHandler)
	pub("GET /api/points/rank", s.pointsLeaderboardHandler)
	pri("GET /api/points", s.pointsDetail)
	pri("GET /api/points/log", s.pointsDetail)
	pri("GET /api/points/checkin", s.checkinStatus)
	pri("POST /api/points/checkin", s.checkin)
	pri("POST /api/points/online", s.onlineHeartbeat)
	pri("GET /api/points/online", s.onlineSummary)

	// 成就与等级
	pub("GET /api/levels", s.levelTable)
	pri("GET /api/achievements", s.achievements)
	pri("GET /api/levels/me", s.levelMe)
	pri("POST /api/levels/buy", s.levelBuy)

	// 积分商城
	pub("GET /api/shop", s.shopList)
	pri("POST /api/shop/redeem", s.shopRedeem)
	pri("GET /api/shop/orders", s.shopOrders)

	// AI 问答与解析
	pri("POST /api/ai/ask", s.aiAsk)
	pri("GET /api/ai/history", s.aiHistory)

	// 管理后台：积分规则与商城
	pri("POST /api/admin/points/adjust", s.adminPointsAdjust)
	pri("POST /api/admin/points/grant", s.adminPointsGrant)
	pri("POST /api/admin/points/contest/{id}/rule", s.contestPointsSetRule)
	pri("GET /api/admin/points/contest/{id}/rule", s.contestPointsGetRule)
	pri("POST /api/admin/points/contest/{id}/apply", s.contestPointsApply)
	pri("GET /api/admin/shop", s.shopAdminList)
	pri("POST /api/admin/shop", s.shopCreate)
	pri("PUT /api/admin/shop/{id}", s.shopUpdate)
	pri("DELETE /api/admin/shop/{id}", s.shopDelete)

	// 未匹配的 /api/ 与 /auth/ 路径给出 JSON 404，避免注册下线等业务变更后
	// 调用方拿到静态页或 HTML 404 而无法解析。更具体的路由优先，不会被吞。
	mux.HandleFunc("/api/{path...}", func(w http.ResponseWriter, r *http.Request) {
		Fail(w, http.StatusNotFound, "接口不存在")
	})
	mux.HandleFunc("/auth/{path...}", func(w http.ResponseWriter, r *http.Request) {
		Fail(w, http.StatusNotFound, "登录入口不存在")
	})
	return mux
}

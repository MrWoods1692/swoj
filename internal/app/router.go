package app

import (
	"net/http"
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
		token := bearerToken(r)
		if token == "" {
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
	// base 是全局链：recovery → cors → ipBlock → csrf → accessLog。
	// accessLog 放最内层（最后加），这样写日志时 claims 已由 auth 注入、status_code 已确定。
	// 对公开路由（未登录）accessLog 也会写入，user_id/username 为空——便于审计全站访问。
	base := []func(HandlerFunc) HandlerFunc{
		recoveryMiddleware, corsMiddleware, ipBlockMiddleware(s.db), csrfMiddleware(),
		accessLogMiddleware(s.db, s.cfg.JWTSecret),
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
	// 协议与隐私政策：版本比对 + 记录同意时间，前端据此弹出确认框。
	pub("GET /api/terms", s.termsInfo)
	pri("POST /api/terms/accept", s.termsAccept)
	// 公开个人主页：任何访客可查看，前端外链 /profile/{id} 直接渲染此数据。
	pub("GET /api/users/{id}/homepage", s.userHomepage)
	// 收藏：题目与训练计划共用一组端点，type 为 problem 或 plan。
	pri("POST /api/favorites/{type}/{id}", s.favoriteToggle)
	pri("GET /api/favorites/{type}", s.favoriteList)

	// 公告：公开只读，草稿与增删改均在管理端。
	pub("GET /api/notices", s.noticeList)
	pub("GET /api/notices/{id}", s.noticeDetail)
	pri("GET /api/admin/notices", s.noticeAdminList)
	pri("POST /api/admin/notices", s.noticeCreate)
	pri("PUT /api/admin/notices/{id}", s.noticeUpdate)
	pri("DELETE /api/admin/notices/{id}", s.noticeDelete)

	// 资料：公开只读 + 元数据；增删改仅老师与管理员（handler 内 requireEditorClaims 校验）。
	pub("GET /api/materials/meta", s.materialsMeta)
	pub("GET /api/materials", s.materialsList)
	pub("GET /api/materials/{id}", s.materialDetail)
	pri("GET /api/admin/materials", s.materialsAdminList)
	pri("POST /api/admin/materials", s.materialCreate)
	pri("PUT /api/admin/materials/{id}", s.materialUpdate)
	pri("DELETE /api/admin/materials/{id}", s.materialDelete)
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

	// 服务状态（公开）与测评机实时状态/配置参数展示，服务端健康检查
	pub("GET /api/status", s.serviceStatus)
	pub("GET /api/judge/info", s.judgeInfo)
	pri("GET /api/admin/judge/config", s.judgeConfigGet)
	pri("PUT /api/admin/judge/config", s.judgeConfigSet)

	// 全站统计：公开概览供首页，明细面板限管理员。
	pub("GET /api/stats/site", s.statsSite)
	pri("GET /api/admin/stats", s.statsAdmin)

	// 更新日志：公开列表与详情，管理后台维护。
	pub("GET /api/changelog", s.changelogList)
	pub("GET /api/changelog/kinds", s.changelogMeta)
	pub("GET /api/changelog/{id}", s.changelogDetail)
	pri("GET /api/admin/changelog", s.changelogAdminList)
	pri("POST /api/admin/changelog", s.changelogCreate)
	pri("PUT /api/admin/changelog/{id}", s.changelogUpdate)
	pri("DELETE /api/admin/changelog/{id}", s.changelogDelete)

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
	pri("GET /api/admin/logs", s.adminLogs)
	pri("GET /api/admin/logs/users", s.logUserIDResolver)
	// 个人日志：登录用户查看自己的操作记录（服务端强制绑定当前 uid，忽略 user_id 参数）。
	pri("GET /api/logs", s.myLogs)
	// 日志辅助端点：动作下拉与每日趋势。登录即可用，作用域由角色决定——
	// 管理员看全站，普通用户仅看自己产生过的动作与自己记录的趋势。
	// 放在 /api/logs/* 而非 /api/admin/logs/*，让个人日志页复用同一入口。
	pri("GET /api/logs/actions", s.logActionOptions)
	pri("GET /api/logs/daily", s.logRangeByDay)

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
	pri("GET /api/ai/stats", s.aiStats)
	// 管理员 AI 配置（token / system_prompt / provider）
	pri("GET /api/admin/ai-config", s.aiConfigGet)
	pri("PUT /api/admin/ai-config", s.aiConfigSave)
	pri("GET /api/admin/ai-stats", s.aiAdminStats)

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

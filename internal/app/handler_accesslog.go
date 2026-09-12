package app

import (
	"net/http"
)

// statusRecorder 捕获响应状态码，用于访问日志的 status_code 字段。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// accessLogMiddleware 记录所有进入 API 的请求。
//
// 位置：base 中间件链末端（最内层），在 authMiddleware.Wrap 之后、真正业务 handler 之前。
// 选择内层的原因：外层无法读到 statusRecorder 写入的 status_code。
// 注意：authMiddleware 通过 AuthContext 传入的是**新的 request**，本层看到的 r 仍是原请求，
// 因此不能依赖 ClaimsFrom(r)——必须自行解码 bearerToken 得到 user_id。
//
// 字段口径：
//   - action = "api:" + Method + " " + Path（与业务 logOp 使用的语义动作区分）
//   - target = 空串（业务侧才有 target，例如 problem id）
//   - detail = 空串（业务侧才有语义描述）
//   - path = r.URL.Path（含查询串前缀）
//   - status_code = 实际响应状态码
//
// 未登录请求（含公开端点）同样记录，user_id/username 为空。这样管理员可以审计
// 全站访问，也便于定位未登录用户的失败请求来源。
//
// 日志失败静默：访问日志是审计辅助，失败不能影响业务响应。
func accessLogMiddleware(db *DB, jwtSecret string) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: 0}
			ip := clientIP(r)
			next(rec, r)

			code := rec.status
			if code == 0 {
				code = http.StatusOK
			}
			uid, uname := int64(0), ""
			// 从 bearerToken 解码得到 user_id（authMiddleware 的 AuthContext 是新的 r，本层看不到）。
			// 失败时保持 uid=0，不影响主流程。
			if tok := bearerToken(r); tok != "" {
				if c, err := ParseToken(jwtSecret, tok); err == nil {
					uid = c.UserID
					_ = db.QueryRow(`SELECT username FROM users WHERE id=?`, uid).Scan(&uname)
				}
			}
			action := "api:" + r.Method + " " + r.URL.Path
			_, _ = db.Exec(`INSERT INTO operation_logs(user_id,username,action,target,detail,ip,path,status_code)
				VALUES(?,?,?,?,?,?,?,?)`, uid, uname, action, "", "", ip, r.URL.Path, code)
		}
	}
}

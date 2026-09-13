package app

import (
	"database/sql"
	"net/http"
	"time"
)

// 账号只能通过校园墙 OAuth 建立（见 handler_oauth.go），
// 本文件只保留登录态相关端点：登出与当前用户查询。

// logout 清理令牌 Cookie。
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "swoj_token", Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "swoj_csrf", Value: "", Path: "/", MaxAge: -1})
	OK(w, nil)
}

// 协议版本：写入 users.terms_accepted_at，前端据此判定是否需要重新确认。
// 提升版本号即可让全体用户在下次登录时重新看到条款。
const TermsVersion = "2026-09"

// optionalClaims 解析请求中的登录态但不强制：无令牌或令牌失效时返回 nil，
// 供「匿名也可访问、登录后返回个性化结果」的公开端点使用。
func (s *Server) optionalClaims(r *http.Request) *Claims {
	if c, ok := ClaimsFrom(r); ok {
		return c
	}
	token := bearerToken(r)
	if token == "" {
		return nil
	}
	claims, err := ParseToken(s.cfg.JWTSecret, token)
	if err != nil {
		return nil
	}
	return claims
}

// termsAccept 记录当前用户同意协议与隐私政策的时间戳。
func (s *Server) termsAccept(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r)
	if !ok {
		Fail(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var ver struct {
		Version string `json:"version"`
	}
	if err := decode(r, &ver); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if ver.Version == "" || ver.Version != TermsVersion {
		Fail(w, http.StatusBadRequest, "协议版本无效")
		return
	}
	stamp := TermsVersion
	if _, err := s.db.Exec(`UPDATE users SET terms_accepted_at=? WHERE id=?`, stamp, claims.UserID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "terms_accept", TermsVersion, "同意用户协议与隐私政策", clientIP(r))
	OK(w, map[string]any{"terms_accepted_at": stamp, "version": TermsVersion})
}

// termsInfo 返回当前协议版本，供前端在不打扰用户的前提下判断是否需要重新确认。
// 未登录也放行，但此时无法比对同意状态，仅返回版本。
func (s *Server) termsInfo(w http.ResponseWriter, r *http.Request) {
	var accepted string
	if claims := s.optionalClaims(r); claims != nil {
		_ = s.db.QueryRow(`SELECT terms_accepted_at FROM users WHERE id=?`, claims.UserID).Scan(&accepted)
	}
	OK(w, map[string]any{
		"version":     TermsVersion,
		"accepted_at": accepted,
		"need_accept": accepted != TermsVersion,
	})
}

// me 返回当前登录用户信息。
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r)
	if !ok {
		Fail(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var u User
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature, website, background, qq,
		problem_count, points, can_submit, terms_accepted_at, created_at, last_login_at FROM users WHERE id=?`, claims.UserID).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.Website, &u.Background, &u.QQ,
			&u.ProblemCount, &u.Points, &u.CanSubmit, &u.TermsAcceptedAt,
			(*time.Time)(&u.CreatedAt), (*time.Time)(&u.LastLoginAt))
	if err == sql.ErrNoRows {
		Fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 头像列不自填，响应层补齐 QQ 派生值：否则顶栏与个人中心只能看到字母占位。
	if u.Avatar == "" {
		u.Avatar = qqAvatarURL(u.QQ)
	}
	OK(w, u)
}

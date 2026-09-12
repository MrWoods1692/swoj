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

// me 返回当前登录用户信息。
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r)
	if !ok {
		Fail(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var u User
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature, website, background, qq,
		problem_count, points, can_submit, created_at, last_login_at FROM users WHERE id=?`, claims.UserID).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.Website, &u.Background, &u.QQ,
			&u.ProblemCount, &u.Points, &u.CanSubmit, (*time.Time)(&u.CreatedAt), (*time.Time)(&u.LastLoginAt))
	if err == sql.ErrNoRows {
		Fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, u)
}

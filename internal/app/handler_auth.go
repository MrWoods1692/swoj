package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

// LoginReq 登录请求。
// 普通用户已下线自建账号，此入口仅保留给管理员控制台使用（role=super）。
type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// login 校验口令、签发令牌与 CSRF，记录最近登录时间。
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	var req LoginReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !s.RateLimiter.Allow(ip) {
		Fail(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}

	var u User
	var password, createdAt, lastLogin string
	err := s.db.QueryRow(`SELECT id, username, password, email, realname, role, school, avatar, signature,
		problem_count, can_submit, created_at, last_login_at FROM users WHERE username=?`, req.Username).
		Scan(&u.ID, &u.Username, &password, &u.Email, &u.RealName, &u.Role, &u.School,
			&u.Avatar, &u.Signature, &u.ProblemCount, &u.CanSubmit, &createdAt, &lastLogin)
	if err != nil {
		Fail(w, http.StatusBadRequest, ErrInvalidCredentials.Error())
		return
	}
	if u.Role != "super" {
		Fail(w, http.StatusBadRequest, "请使用校园墙登录")
		return
	}
	if !verifyPassword(req.Password, password) {
		Fail(w, http.StatusBadRequest, ErrInvalidCredentials.Error())
		return
	}

	claims, err := SignToken(s.cfg.JWTSecret, s.cfg.JWTLife, u.ID, u.Role)
	if err != nil {
		Fail(w, http.StatusInternalServerError, "令牌签发失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "swoj_token", Value: claims, Path: "/", MaxAge: 86400,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	csrf := IssueCSRF(w)
	_, _ = s.db.Exec(`UPDATE users SET last_login_at=datetime('now') WHERE id=?`, u.ID)
	logOp(s.db, &Claims{UserID: u.ID, Role: u.Role}, "login", u.Username, "登录成功", ip)
	s.refreshAchievements(u.ID)

	OK(w, map[string]any{"token": claims, "csrf": csrf, "user": u,
		"created_at": createdAt, "last_login_at": lastLogin})
}

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
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature, problem_count,
		points, can_submit, created_at, last_login_at FROM users WHERE id=?`, claims.UserID).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
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

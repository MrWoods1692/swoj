package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

// LoginReq 登录请求。
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

	OK(w, map[string]any{"token": claims, "csrf": csrf, "user": u,
		"created_at": createdAt, "last_login_at": lastLogin})
}

// RegisterReq 注册请求。
type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Confirm  string `json:"confirm"`
	Email    string `json:"email"`
}

// register 创建新账号；用户名需全局唯一。
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Username) > 24 {
		Fail(w, http.StatusBadRequest, "用户名长度需在 3-24 个字符之间")
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 64 {
		Fail(w, http.StatusBadRequest, "密码长度需在 6-64 个字符之间")
		return
	}
	if req.Password != req.Confirm {
		Fail(w, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=?`, req.Username).Scan(&n)
	if n > 0 {
		Fail(w, http.StatusBadRequest, ErrUsernameExists.Error())
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		Fail(w, http.StatusInternalServerError, "注册失败")
		return
	}
	res, err := s.db.Exec(`INSERT INTO users(username,password,email,role) VALUES(?,?,?,'user')`,
		req.Username, hash, req.Email)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	token, _ := SignToken(s.cfg.JWTSecret, s.cfg.JWTLife, id, "user")
	http.SetCookie(w, &http.Cookie{Name: "swoj_token", Value: token, Path: "/",
		MaxAge: 86400, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	OK(w, map[string]any{"token": token, "csrf": IssueCSRF(w), "id": id})
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

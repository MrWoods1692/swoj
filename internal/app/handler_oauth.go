package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Campux OAuth 登录。账号只能通过校园墙授权建立：首次授权自动创建账号；
// 库里第一个完成授权的账号自动成为管理员（role=super），其余为普通用户。
// 新账号处于未完善资料状态（can_submit=0），补全姓名与个人主页后才算完成注册并解锁提交。
const oauthProvider = "campux"

// 真实姓名必须是 3-4 个汉字，与「完善个人主页」的判定口径一致。
var cnRealname = regexp.MustCompile(`^[\x{4e00}-\x{9fff}]{3,4}$`)

const (
	cfgOAuthSecret = "oauth_secret"
	cfgOAuthClient = "oauth_client_id"
)

// oauthConfig 取 OAuth 参数，优先 admin_configs（管理后台可随时改，无需发版），
// 未配置时回落到环境变量加载的默认值。
func (s *Server) oauthConfig() OAuthConfig {
	cfg := s.cfg.OAuth
	if v := s.configValue(cfgOAuthSecret); v != "" {
		cfg.Secret = v
	}
	if v := s.configValue(cfgOAuthClient); v != "" {
		cfg.ClientID = v
	}
	return cfg
}

// configValue 读取单条系统配置，未设置返回空串。
func (s *Server) configValue(key string) string {
	var v string
	_ = s.db.QueryRow(`SELECT value FROM admin_configs WHERE key=?`, key).Scan(&v)
	return v
}

// OAuthRedirect 是回调后跳回前端的地址。只允许站内相对路径，避免开放重定向。
const OAuthRedirect = "/oauth/callback"

// oauthRedirectTo 合并跳转地址，非法值回落到默认地址。
func oauthRedirectTo(w http.ResponseWriter, r *http.Request) string {
	target := OAuthRedirect
	if q := r.URL.Query().Get("redirect"); q != "" {
		u, err := url.Parse(q)
		if err == nil && !u.IsAbs() && strings.HasPrefix(u.Path, "/") {
			target = q
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "swoj_oauth_redirect", Value: target,
		Path: "/", MaxAge: 3600, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return target
}

// oauthCookie 把带属性的 Cookie 写入响应。maxAge 单位秒。
func oauthCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/",
		MaxAge: maxAge, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// oauthAuthorize 生成 state 与 PKCE 挑战，跳转校园墙授权页。
// 用 HTTP 302 而非 JSON，浏览器与前端跳转共用这一个入口。
func (s *Server) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	cfg := s.oauthConfig()
	if !cfg.OAuthEnabled() {
		Fail(w, http.StatusServiceUnavailable, "校园墙登录未配置")
		return
	}
	state := hexID()
	verifier := hexID()
	challenge := sha256b64(verifier)
	oauthRedirectTo(w, r)
	oauthCookie(w, "swoj_oauth_state", state, 600)
	oauthCookie(w, "swoj_oauth_verifier", verifier, 600)

	// 本地联调模式（SWOJ_OAUTH_MOCK=1）：跳过外部授权页，直接跳回本站回调；
	// state 与 verifier 照常签发与校验，验证的是同一条链路。
	if cfg.Mock && r.URL.Query().Get("mock") == "1" {
		cb := url.Values{}
		cb.Set("state", state)
		cb.Set("mock", "1")
		for _, k := range []string{"mockname", "mockqq"} {
			if v := r.URL.Query().Get(k); v != "" {
				cb.Set(k, v)
			}
		}
		http.Redirect(w, r, cfg.Callback+"?"+cb.Encode(), http.StatusFound)
		return
	}

	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.OAuthCallbackURL())
	q.Set("response_type", "code")
	q.Set("scope", cfg.Scope)
	q.Set("state", state)
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", challenge)
	http.Redirect(w, r, cfg.AuthURL+"?"+q.Encode(), http.StatusFound)
}

// oauthCallback 校验 state 与 PKCE，换取令牌并拉取用户信息。
// 回调后必须签发 JWT 与 CSRF：登录发生在导航请求里，前端不会自动重新拉 CSRF。
func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	cfg := s.oauthConfig()
	if !cfg.OAuthEnabled() {
		Fail(w, http.StatusServiceUnavailable, "校园墙登录未配置")
		return
	}
	stored, _ := r.Cookie("swoj_oauth_state")
	verifier, _ := r.Cookie("swoj_oauth_verifier")
	q := r.URL.Query()
	isMock := q.Get("mock") == "1"

	redirect, _ := r.Cookie("swoj_oauth_redirect")
	target := OAuthRedirect
	if redirect != nil && redirect.Value != "" {
		target = redirect.Value
	}

	// 授权被拒绝、账号未登录或 redirect_uri 未注册时，Campux 会带 error 回跳
	// 而不给 code。先确认 state 再展示对方给的失败原因。
	if e := q.Get("error"); e != "" {
		desc := q.Get("error_description")
		if desc == "" {
			desc = e
		}
		Fail(w, http.StatusUnauthorized, "校园墙授权未完成："+desc)
		return
	}

	if stored == nil || q.Get("state") == "" || stored.Value != q.Get("state") {
		// 三条分支的文案分开写：这个分支不会出现在前端任何页面上，
		// 只会在登录跳回时直出 JSON，必须能自己说明下一步怎么做。
		if stored == nil {
			Fail(w, http.StatusBadRequest,
				"未找到授权凭证，请回到本站首页重新点击「校园墙登录」")
			return
		}
		Fail(w, http.StatusBadRequest,
			"授权状态已过期或与当前页面不一致，请刷新页面后重新授权")
		return
	}
	if stored != nil {
		oauthCookie(w, "swoj_oauth_state", "", -1)
	}
	if verifier != nil {
		oauthCookie(w, "swoj_oauth_verifier", "", -1)
	}
	if redirect != nil {
		oauthCookie(w, "swoj_oauth_redirect", "", -1)
	}

	// 本地联调分支：建号/登录与真实回调同路径，但以 JSON 返回令牌供脚本验证。
	if isMock {
		if !cfg.Mock {
			Fail(w, http.StatusForbidden, "本地联调模式未开启")
			return
		}
		name := q.Get("mockname")
		if name == "" {
			name = "本地联调"
		}
		qqid := q.Get("mockqq")
		if qqid == "" {
			qqid = "13800138000"
		}
		s.oauthIssue(w, r, name, qqid, target, true)
		return
	}

	access, err := oauthExchangeToken(cfg, q.Get("code"), verifier.Value)
	if err != nil {
		Fail(w, http.StatusUnauthorized, "登录失败："+err.Error())
		return
	}
	info, err := oauthUserInfo(cfg, access)
	if err != nil {
		Fail(w, http.StatusInternalServerError, "获取校园墙信息失败："+err.Error())
		return
	}
	s.oauthIssue(w, r, info.Name, info.QQ, target, false)
}

// oauthIssue 按 oauth_id 查找或创建用户，签发令牌。asJSON 为 true 时直接返回令牌
// （本地联调），否则把登录态写入 Cookie 后 302 跳回前端页面。
func (s *Server) oauthIssue(w http.ResponseWriter, r *http.Request, username, qq, redirect string, asJSON bool) {
	username = strings.TrimSpace(username)
	if len([]rune(username)) < 3 || len([]rune(username)) > 24 {
		Fail(w, http.StatusBadRequest, "校园墙账号名称不合法")
		return
	}
	var id int64
	var role string
	_ = s.db.QueryRow(`SELECT id, role FROM users WHERE oauth_provider=? AND oauth_id=?`,
		oauthProvider, qq).Scan(&id, &role)
	if id == 0 {
		// 库里还没有任何校园墙账号 → 本次登录者是首位，直接授予管理员；
		// 按 OAuth 绑定数而非 super 数判定，避免遗留密码账号占住管理员位。
		var bound int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE oauth_provider<>''`).Scan(&bound)
		if bound == 0 {
			role = "super"
		} else {
			role = "user"
		}
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=?`, username).Scan(&n)
		if n > 0 {
			// 用户名已被占用：用 OAuth ID 生成不冲突的账号名，保证建号不因重名失败。
			username = "campux_" + qq
		}
		res, err := s.db.Exec(`INSERT INTO users(username, oauth_provider, oauth_id, oauth_name, role, qq, can_submit)
			VALUES(?,?,?,?,?,?,0)`, username, oauthProvider, qq, username, role, qq)
		if err != nil {
			// 并发首次登录：按 OAuth 绑定回查，避免重复建号。
			if err2 := s.db.QueryRow(`SELECT id, role FROM users WHERE oauth_provider=? AND oauth_id=?`,
				oauthProvider, qq).Scan(&id, &role); err2 != nil {
				Fail(w, http.StatusInternalServerError, "创建账号失败")
				return
			}
		} else {
			id, _ = res.LastInsertId()
		}
	}

	var u User
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature, website, background, qq,
		problem_count, points, level, can_submit, terms_accepted_at, created_at, last_login_at FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.Website, &u.Background, &u.QQ,
			&u.ProblemCount, &u.Points, &u.Level, &u.CanSubmit, &u.TermsAcceptedAt,
			(*time.Time)(&u.CreatedAt), (*time.Time)(&u.LastLoginAt))
	if err != nil {
		Fail(w, http.StatusInternalServerError, "账号不存在")
		return
	}
	// 与 userByID / userHomepage 一致：头像列不自填，响应层补齐 QQ 派生值。
	if u.Avatar == "" {
		u.Avatar = qqAvatarURL(u.QQ)
	}

	token, err := SignToken(s.cfg.JWTSecret, s.cfg.JWTLife, u.ID, u.Role)
	if err != nil {
		Fail(w, http.StatusInternalServerError, "令牌签发失败")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "swoj_token", Value: token, Path: "/",
		MaxAge: int(s.cfg.JWTLife.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	csrf := IssueCSRF(w)
	_, _ = s.db.Exec(`UPDATE users SET last_login_at=datetime('now') WHERE id=?`, u.ID)
	s.refreshAchievements(u.ID)
	logOp(s.db, &Claims{UserID: u.ID, Role: u.Role}, "oauth_login", u.Username, "校园墙登录", clientIP(r))

	if asJSON {
		OK(w, map[string]any{"token": token, "csrf": csrf, "user": u})
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "swoj_oauth_csrf", Value: csrf, Path: "/",
		MaxAge: int(s.cfg.JWTLife.Seconds()), HttpOnly: false, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "swoj_oauth_user", Value: u.Username, Path: "/",
		MaxAge: int(s.cfg.JWTLife.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, redirect, http.StatusFound)
}

// oauthExchangeToken 用授权码换取访问令牌，PKCE 校验码随请求提交。
func oauthExchangeToken(cfg OAuthConfig, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.Secret},
		"code":          {code},
		"redirect_uri":  {cfg.OAuthCallbackURL()},
	}
	if verifier != "" {
		form.Set("code_verifier", verifier)
	}
	resp, err := oauthPost(cfg.TokenURL, form)
	if err != nil {
		return "", err
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", errors.New("令牌响应解析失败")
	}
	if out.AccessToken == "" {
		return "", errors.New("未返回访问令牌")
	}
	return out.AccessToken, nil
}

// oauthUserInfo 拉取校园墙用户信息。
// Campux /oauth/userinfo 的字段语义与直觉相反：name 是数字账号（QQ 号），
// username 才是昵称。必须按这个顺序映射，否则用户名与 QQ 号会互换显示。
type oauthProfile struct {
	Name string
	QQ   string
}

func oauthUserInfo(cfg OAuthConfig, access string) (oauthProfile, error) {
	body, err := oauthGet(cfg.UserInfoURL, access)
	if err != nil {
		return oauthProfile{}, err
	}
	var out struct {
		Name     string `json:"name"`     // 数字账号 / QQ 号
		Username string `json:"username"` // 昵称
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return oauthProfile{}, errors.New("用户信息解析失败")
	}
	display := out.Username
	if display == "" {
		display = out.Name
	}
	qq := out.Name
	if qq == "" {
		qq = out.Username
	}
	return oauthProfile{Name: display, QQ: qq}, nil
}

// oauthPost 发起 POST 表单请求并返回响应体。
func oauthPost(endpoint string, form url.Values) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("授权服务器返回 " + intStr(resp.StatusCode))
	}
	return body, nil
}

// oauthGet 带 Bearer 令牌发起 GET 并返回响应体。
func oauthGet(endpoint, access string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("授权服务器返回 " + intStr(resp.StatusCode))
	}
	return body, nil
}

// readAll 读取响应体，上限 1MB 防止异常响应占内存。
func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, 1<<20))
}

// intStr 把状态码转成字符串，避免错误信息里混用类型。
func intStr(n int) string {
	return strconv.Itoa(n)
}

// sha256b64 计算摘要并转成 URL 安全编码，用于 PKCE code_challenge。
func sha256b64(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

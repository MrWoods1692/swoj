package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LogEntry 操作日志条目，全局与个人日志共用。
type LogEntry struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	CreatedAt  string `json:"created_at"`
}

// logFilters 通用日志查询过滤。
//
// 时间范围按 SQLite 存储格式比较（'YYYY-MM-DD HH:MM:SS' 字典序），
// 前端可传 start/end 两个 ISO 或本地时间字符串；服务端不额外解析，直接比较。
type logFilters struct {
	UID      int64  // 按用户过滤；0 表示不过滤
	Action   string // 按动作子串过滤
	Path     string // 按路径子串过滤
	From     string // created_at >= From
	To       string // created_at <= To
	Page     int
	PageSize int
}

// parseLogFilters 解析查询参数为过滤器，含默认值与边界收敛。
func parseLogFilters(q map[string][]string) logFilters {
	f := logFilters{Page: 1, PageSize: 20}
	if v := q["page"]; len(v) > 0 {
		f.Page = atoiDefault(v[0], 1)
	}
	if v := q["page_size"]; len(v) > 0 {
		f.PageSize = atoiDefault(v[0], 20)
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}
	if v := q["user_id"]; len(v) > 0 {
		f.UID = int64(atoiDefault(v[0], 0))
	}
	if v := q["action"]; len(v) > 0 {
		f.Action = strings.TrimSpace(v[0])
	}
	if v := q["path"]; len(v) > 0 {
		f.Path = strings.TrimSpace(v[0])
	}
	if v := q["from"]; len(v) > 0 {
		f.From = strings.TrimSpace(v[0])
	}
	if v := q["to"]; len(v) > 0 {
		f.To = strings.TrimSpace(v[0])
	}
	return f
}

// scanLogs 执行带过滤条件的分页查询，返回 (行, 总数)。
//
// 过滤条件按 user_id → action → path → from → to 追加，全部使用参数化查询，
// 不拼接用户输入到 SQL 文本里。
func scanLogs(db *DB, f logFilters) ([]LogEntry, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.UID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, f.UID)
	}
	if f.Action != "" {
		where = append(where, "action LIKE ?")
		args = append(args, "%"+f.Action+"%")
	}
	if f.Path != "" {
		where = append(where, "path LIKE ?")
		args = append(args, "%"+f.Path+"%")
	}
	if f.From != "" {
		where = append(where, "created_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "created_at <= ?")
		args = append(args, f.To)
	}
	whereSQL := strings.Join(where, " AND ")
	totalSQL := `SELECT COUNT(*) FROM operation_logs WHERE ` + whereSQL
	var total int
	if err := db.QueryRow(totalSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT id, user_id, username, action, target, detail, ip, path, status_code, created_at
		FROM operation_logs WHERE ` + whereSQL + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		var createdAt any
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &e.Target,
			&e.Detail, &e.IP, &e.Path, &e.StatusCode, &createdAt); err != nil {
			continue
		}
		e.CreatedAt = fmtCreatedAt(createdAt)
		list = append(list, e)
	}
	return list, total, nil
}

// fmtCreatedAt 把 SQLite DATETIME 返回值转成字符串。
func fmtCreatedAt(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return ""
	}
}

// logSummary 汇总统计，用于日志页顶部指标。
type logSummary struct {
	Total       int `json:"total"`
	Today       int `json:"today"`
	Errors30d   int `json:"errors_30d"`
	DistinctU   int `json:"distinct_users"`
	DistinctAct int `json:"distinct_actions"`
}

// summaryLogs 返回日志总量与几个常用口径的辅助指标。
//   - total: 全表总量
//   - today: 近 24 小时内的条目数（用 SQLite datetime('now','-1 day') 计算，与服务端时区一致）
//   - errors_30d: 近 30 天 status_code >= 400 的条目数（访问日志才有 status_code，业务日志默认 0）
//   - distinct_users: 有日志的用户数
//   - distinct_actions: 出现的不同 action 数
func summaryLogs(db *DB) logSummary {
	s := logSummary{}
	_ = db.QueryRow(`SELECT COUNT(*) FROM operation_logs`).Scan(&s.Total)
	_ = db.QueryRow(`SELECT COUNT(*) FROM operation_logs
		WHERE created_at >= datetime('now','-1 day')`).Scan(&s.Today)
	_ = db.QueryRow(`SELECT COUNT(*) FROM operation_logs
		WHERE status_code >= 400 AND created_at >= datetime('now','-30 day')`).Scan(&s.Errors30d)
	_ = db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM operation_logs WHERE user_id > 0`).Scan(&s.DistinctU)
	_ = db.QueryRow(`SELECT COUNT(DISTINCT action) FROM operation_logs`).Scan(&s.DistinctAct)
	return s
}

// adminLogs 管理员视角的全局日志。支持 user_id / action / path / from / to 过滤。
//
// 与旧的 opLogs 区别：
//   - 旧 opLogs 只查无过滤的分页；已被本 handler 取代
//   - 本 handler 保留原有响应结构（{data:{page,page_size,total,list}}）
func (s *Server) adminLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	f := parseLogFilters(r.URL.Query())
	list, total, err := scanLogs(s.db, f)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{
		"page":      f.Page,
		"page_size": f.PageSize,
		"total":     total,
		"list":      list,
		"summary":   summaryLogs(s.db),
	})
}

// myLogs 当前登录用户的个人日志。
//
// 与 adminLogs 差别只在 WHERE user_id = ?，且**不接受 user_id 查询参数**——
// 用户不能通过伪造参数查看他人日志。管理员查他人请用 adminLogs。
func (s *Server) myLogs(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	// 用户只允许通过 user_id 之外的字段过滤，防止越权。
	delete(q, "user_id")
	f := parseLogFilters(q)
	f.UID = claims.UserID
	list, total, err := scanLogs(s.db, f)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{
		"page":      f.Page,
		"page_size": f.PageSize,
		"total":     total,
		"list":      list,
		"summary":   summaryLogs(s.db),
	})
}

// logActionOptions 返回出现的动作列表（用于前端过滤下拉），取最近 200 个不同 action。
// 权限：管理员看全站动作；普通用户看自己产生过的动作。
// 前端个人日志页也要下拉选项，因此不能强制管理员——改为按用户作用域返回。
func (s *Server) logActionOptions(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	q := `SELECT DISTINCT action FROM operation_logs WHERE 1=1`
	args := []any{}
	if !RequireRole(claims, "admin", "super", "superadmin") {
		// 非管理员仅返回自己的动作，避免越权发现全站动作词表。
		q = `SELECT DISTINCT action FROM operation_logs WHERE user_id = ?`
		args = append(args, claims.UserID)
	}
	q += " ORDER BY action LIMIT 200"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err == nil {
			list = append(list, a)
		}
	}
	OK(w, list)
}

// logRangeByDay 按日期分组的每日日志条目数，用于趋势图。
//
// 作用域：管理员看全站趋势；普通用户仅统计自己产生的条目。
// 默认取近 30 天；可通过 days 覆盖（最大 365）。
type dayCount struct {
	Date string `json:"date"`
	Count int   `json:"count"`
}

func (s *Server) logRangeByDay(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	days := atoiDefault(r.URL.Query().Get("days"), 30)
	if days < 1 || days > 365 {
		days = 30
	}
	q := `SELECT date(created_at) AS d, COUNT(*) AS c FROM operation_logs
		WHERE created_at >= datetime('now', ?)`
	args := []any{"-" + strconv.Itoa(days) + " day"}
	if !RequireRole(claims, "admin", "super", "superadmin") {
		q = `SELECT date(created_at) AS d, COUNT(*) AS c FROM operation_logs
			WHERE created_at >= datetime('now', ?) AND user_id = ?`
		args = append(args, claims.UserID)
	}
	q += " GROUP BY d ORDER BY d"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []dayCount{}
	for rows.Next() {
		var d dayCount
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			continue
		}
		list = append(list, d)
	}
	OK(w, list)
}

// logUserIDResolver 供前端"按用户名筛选"的辅助端点，
// 返回匹配到的用户 ID 与用户名，避免前端需要额外调用 /api/admin/users。
func (s *Server) logUserIDResolver(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		OK(w, []LogEntry{})
		return
	}
	rows, err := s.db.Query(`SELECT id, username FROM users WHERE username LIKE ? ORDER BY id LIMIT 20`, "%"+q+"%")
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.UserID, &e.Username); err == nil {
			list = append(list, e)
		}
	}
	OK(w, list)
}

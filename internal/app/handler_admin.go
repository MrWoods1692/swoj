package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProblemCreateReq 管理员新建题目的请求体。
type ProblemCreateReq struct {
	Name        string        `json:"name"`
	Difficulty  string        `json:"difficulty"`
	ProblemType string        `json:"problem_type"`
	TimeLimit   int           `json:"time_limit"`
	MemLimit    int           `json:"mem_limit"`
	FileLimit   int           `json:"file_limit"`
	StackLimit  int           `json:"stack_limit"`
	Invisible   bool          `json:"invisible"`
	Content     string        `json:"content"`
	Hint        string        `json:"hint"`
	Tag         string        `json:"tag"`
	Cases       []ProblemCase `json:"cases"`
}

// ProblemCase 一个测试用例。
type ProblemCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// problemCreate 管理员新建题目并写入初始测试用例。
func (s *Server) problemCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req ProblemCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		Fail(w, http.StatusBadRequest, "题目名称不能为空")
		return
	}
	if req.TimeLimit <= 0 {
		req.TimeLimit = 1000
	}
	if req.MemLimit <= 0 {
		req.MemLimit = 256
	}
	if req.Difficulty == "" {
		req.Difficulty = "Easy"
	}
	if req.ProblemType == "" {
		req.ProblemType = "OJ"
	}
	res, err := s.db.Exec(`INSERT INTO problems(name, difficulty, problem_type, time_limit, mem_limit,
		file_limit, stack_limit, invisible, content, hint) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		req.Name, req.Difficulty, req.ProblemType, req.TimeLimit, req.MemLimit,
		req.FileLimit, req.StackLimit, boolInt(req.Invisible), req.Content, req.Hint)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()

	for i, c := range req.Cases {
		if strings.TrimSpace(c.Input) == "" && strings.TrimSpace(c.Output) == "" {
			continue
		}
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO cases(problem_id, index_no, input, output) VALUES(?,?,?,?)`,
			id, i+1, c.Input, c.Output)
	}
	logOp(s.db, claims, "problem_create", strconv.FormatInt(id, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": id, "cases": len(req.Cases)})
}

// problemUpdate 管理员修改题目基本信息与测试用例。
func (s *Server) problemUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "题目编号无效")
		return
	}
	var req ProblemCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := s.db.Exec(`UPDATE problems SET name=?, difficulty=?, time_limit=?, mem_limit=?,
		file_limit=?, stack_limit=?, invisible=?, content=?, hint=? WHERE id=?`,
		req.Name, req.Difficulty, req.TimeLimit, req.MemLimit, req.FileLimit, req.StackLimit,
		boolInt(req.Invisible), req.Content, req.Hint, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 仅当本次请求携带用例时才整体替换，便于只改基本信息。
	if req.Cases != nil {
		_, _ = s.db.Exec(`DELETE FROM cases WHERE problem_id=?`, id)
		for i, c := range req.Cases {
			if strings.TrimSpace(c.Input) == "" && strings.TrimSpace(c.Output) == "" {
				continue
			}
			_, _ = s.db.Exec(`INSERT OR REPLACE INTO cases(problem_id, index_no, input, output) VALUES(?,?,?,?)`,
				id, i+1, c.Input, c.Output)
		}
	}
	logOp(s.db, claims, "problem_update", strconv.FormatInt(id, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// problemDelete 管理员删除题目及其用例与相关提交。
func (s *Server) problemDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "题目编号无效")
		return
	}
	var tx *sql.Tx
	tx, err := s.db.Begin()
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()
	_, _ = tx.Exec(`DELETE FROM submissions WHERE problem_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM cases WHERE problem_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM contest_problems WHERE problem_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM assignment_problems WHERE problem_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM training_records WHERE problem_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM discussions WHERE problem_id=?`, id)
	if _, err := tx.Exec(`DELETE FROM problems WHERE id=?`, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "problem_delete", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id, "deleted": true})
}

// boolInt 把布尔值写为 SQLite 0/1。
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// UserRow 用户管理列表项。
type UserRow struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"realname"`
	Role      string `json:"role"`
	School    string `json:"school"`
	Submitted int    `json:"submitted"`
	Accepted  int    `json:"accepted"`
	CreatedAt string `json:"created_at"`
}

// userList 管理员查看用户。
func (s *Server) userList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT u.id, u.username, u.realname, u.role, u.school,
		(SELECT COUNT(*) FROM submissions s WHERE s.user_id=u.id),
		(SELECT COUNT(*) FROM submissions s WHERE s.user_id=u.id AND s.status=?), u.created_at
		FROM users u ORDER BY id DESC LIMIT 200`, StatusAccepted)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []UserRow{}
	for rows.Next() {
		var u UserRow
		var created time.Time
		if err := rows.Scan(&u.ID, &u.Username, &u.RealName, &u.Role, &u.School, &u.Submitted,
			&u.Accepted, (*time.Time)(&created)); err != nil {
			continue
		}
		u.CreatedAt = created.String()
		list = append(list, u)
	}
	OK(w, list)
}

// userUpdate 管理员调整用户角色、学校与提交权限。
func (s *Server) userUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "用户编号无效")
		return
	}
	var req struct {
		Role      string `json:"role"`
		School    string `json:"school"`
		CanSubmit *bool  `json:"can_submit"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	// SQLite 的空更新不反映真实影响行数，因此先确认行存在再改。
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists == 0 {
		Fail(w, http.StatusNotFound, "用户不存在")
		return
	}
	_, err := s.db.Exec(`UPDATE users SET role=?, school=?, can_submit=? WHERE id=?`,
		req.Role, req.School, boolInt(req.CanSubmit != nil && *req.CanSubmit), id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "user_update", strconv.FormatInt(id, 10), "role="+req.Role, clientIP(r))
	OK(w, map[string]any{"id": id, "role": req.Role})
}

// IPBlockRow IP 封禁列表项。
type IPBlockRow struct {
	ID        int64  `json:"id"`
	IP        string `json:"ip"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
	ExpireAt  string `json:"expire_at"`
}

// ipBlockList 列出当前生效的 IP 封禁。
func (s *Server) ipBlockList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, ip, reason, created_at, expire_at FROM ip_blocks
		WHERE expire_at IS NULL OR expire_at > CURRENT_TIMESTAMP ORDER BY id DESC`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []IPBlockRow{}
	for rows.Next() {
		var it IPBlockRow
		var created time.Time
		var expire sql.NullTime
		if err := rows.Scan(&it.ID, &it.IP, &it.Reason, (*time.Time)(&created), &expire); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		if expire.Valid {
			it.ExpireAt = expire.Time.String()
		}
		list = append(list, it)
	}
	OK(w, list)
}

// ipBlockAdd 封禁指定 IP，可设过期时间；不带过期时间则永久。
func (s *Server) ipBlockAdd(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		IP       string `json:"ip"`
		Reason   string `json:"reason"`
		ExpireAt string `json:"expire_at"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.IP) == "" {
		Fail(w, http.StatusBadRequest, "IP 不能为空")
		return
	}
	var expire any
	if strings.TrimSpace(req.ExpireAt) != "" {
		expire = strings.TrimSpace(req.ExpireAt)
	}
	res, err := s.db.Exec(`INSERT INTO ip_blocks(ip, reason, expire_at) VALUES(?,?,?)
		ON CONFLICT(ip) DO UPDATE SET reason=excluded.reason, expire_at=excluded.expire_at,
		created_at=CURRENT_TIMESTAMP`, strings.TrimSpace(req.IP), req.Reason, expire)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "ip_block_add", req.IP, req.Reason, clientIP(r))
	OK(w, map[string]any{"id": id, "ip": req.IP})
}

// ipBlockRemove 解除 IP 封禁。
func (s *Server) ipBlockRemove(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "封禁编号无效")
		return
	}
	res, err := s.db.Exec(`DELETE FROM ip_blocks WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "封禁记录不存在")
		return
	}
	logOp(s.db, claims, "ip_block_remove", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id, "removed": true})
}

// ConfigRow 系统配置项。
type ConfigRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// configList 读取全部系统配置。
func (s *Server) configList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT key, value FROM admin_configs ORDER BY key`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ConfigRow{}
	for rows.Next() {
		var c ConfigRow
		if err := rows.Scan(&c.Key, &c.Value); err != nil {
			continue
		}
		list = append(list, c)
	}
	OK(w, list)
}

// configSet 写入系统配置，采用 upsert 避免主键冲突。
func (s *Server) configSet(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req []ConfigRow
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req) == 0 {
		Fail(w, http.StatusBadRequest, "配置项不能为空")
		return
	}
	for _, c := range req {
		if strings.TrimSpace(c.Key) == "" {
			Fail(w, http.StatusBadRequest, "配置键不能为空")
			return
		}
		if _, err := s.db.Exec(`INSERT INTO admin_configs(key, value) VALUES(?,?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, c.Key, c.Value); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	logOp(s.db, claims, "config_set", strconv.Itoa(len(req)), "items updated", clientIP(r))
	OK(w, map[string]any{"updated": len(req)})
}

// LogRow 操作日志列表项。
type LogRow struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

// opLogs 分页返回操作日志。
func (s *Server) opLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	size := atoiDefault(q.Get("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM operation_logs`).Scan(&total)

	rows, err := s.db.Query(`SELECT id, username, action, target, detail, ip, created_at
		FROM operation_logs ORDER BY id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []LogRow{}
	for rows.Next() {
		var it LogRow
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Username, &it.Action, &it.Target, &it.Detail, &it.IP,
			(*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		list = append(list, it)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "page_size": size})
}

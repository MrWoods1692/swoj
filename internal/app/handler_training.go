package app

import (
	"net/http"
	"strconv"
	"time"
)

// PlanResp 训练计划详情响应，含完成进度。
type PlanResp struct {
	ID       int64         `json:"id"`
	Name     string        `json:"name"`
	Info     string        `json:"info"`
	Visible  bool          `json:"visible"`
	Total    int           `json:"total"`
	Accepted int           `json:"accepted"`
	Records  []TrainingRec `json:"records"`
}

// TrainingRec 训练计划中的单题进度。
type TrainingRec struct {
	ProblemID   int64  `json:"problem_id"`
	ProblemName string `json:"problem_name"`
	Sequence    int    `json:"sequence"`
	Accepted    bool   `json:"accepted"`
	Times       int    `json:"times"`
}

// PlanCreateReq 训练计划创建请求，仅管理员可用。
type PlanCreateReq struct {
	Name      string `json:"name"`
	Info      string `json:"info"`
	Visible   bool   `json:"visible"`
	ProblemID int64  `json:"problem_id"`
}

// planList 返回全部可见训练计划及其完成进度。
// 注意：SQLite 单连接下不能在 rows 迭代期间发起新查询，
// 因此先扫完基础字段再逐个补进度。
func (s *Server) planList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, info, visible FROM training_plans WHERE visible=1 ORDER BY id DESC`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	var list []PlanResp
	for rows.Next() {
		var p PlanResp
		if err := rows.Scan(&p.ID, &p.Name, &p.Info, &p.Visible); err != nil {
			continue
		}
		p.Records = []TrainingRec{}
		list = append(list, p)
	}
	rows.Close()

	for i := range list {
		p := &list[i]
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM training_records WHERE plan_id=?`, p.ID).Scan(&p.Total)
		if p.Total > 0 {
			_ = s.db.QueryRow(`SELECT COALESCE(SUM(accepted),0) FROM training_records WHERE plan_id=?`, p.ID).Scan(&p.Accepted)
		}
	}
	OK(w, list)
}

// planDetail 返回单个训练计划及其逐题进度。
func (s *Server) planDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "计划编号无效")
		return
	}
	var p PlanResp
	err := s.db.QueryRow(`SELECT id, name, info, visible FROM training_plans WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Info, &p.Visible)
	if err != nil {
		Fail(w, http.StatusNotFound, "训练计划不存在")
		return
	}
	rows, err := s.db.Query(`SELECT problem_id, problem_name, sequence, accepted, times
		FROM training_records WHERE plan_id=? ORDER BY sequence, id`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	p.Records = []TrainingRec{}
	for rows.Next() {
		var rec TrainingRec
		if err := rows.Scan(&rec.ProblemID, &rec.ProblemName, &rec.Sequence, &rec.Accepted, &rec.Times); err != nil {
			continue
		}
		p.Records = append(p.Records, rec)
		p.Total++
		if rec.Accepted {
			p.Accepted++
		}
	}
	OK(w, p)
}

// planCreate 管理员创建训练计划并挂载一道题目。
func (s *Server) planCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if !requireAdmin(w, claims) {
		return
	}
	var req PlanCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM problems WHERE id=?`, req.ProblemID).Scan(&n)
	if n == 0 {
		Fail(w, http.StatusNotFound, "题目不存在")
		return
	}
	res, err := s.db.Exec(`INSERT INTO training_plans(name, info, visible, creator) VALUES(?,?,?,?)`,
		req.Name, req.Info, req.Visible, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	planID, _ := res.LastInsertId()
	var cur int
	seq := 1
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(sequence),0) FROM training_records WHERE plan_id=?`, planID).
		Scan(&cur); err == nil {
		seq = cur + 1
	}
	if _, err := s.db.Exec(`INSERT INTO training_records(plan_id, problem_id, sequence) VALUES(?,?,?)`,
		planID, req.ProblemID, seq); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "plan_create", strconv.FormatInt(planID, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": planID})
}

// submissionList 提交记录列表，支持按用户、题目、状态筛选。
type submissionListReq struct {
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	UserID       int64  `json:"user_id"`
	ProblemID    int64  `json:"problem_id"`
	Status       int    `json:"status"`
	Username     string `json:"username"`
	ContestID    int64  `json:"contest_id"`
	AssignmentID int64  `json:"assignment_id"`
}

// SubListItem 提交列表行。
type SubListItem struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	ProblemID   int64  `json:"problem_id"`
	ProblemName string `json:"problem_name"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	TimeUsed    int    `json:"time_used"`
	MemUsed     int    `json:"mem_used"`
	CreatedAt   string `json:"created_at"`
}

// submissionList 分页返回提交记录，筛选参数只从 URL query 读取。
// 这里不能 decode body：解码出的零值会覆盖 query 中已解析的参数。
func (s *Server) submissionList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := submissionListReq{
		Page:     atoiDefault(q.Get("page"), 1),
		PageSize: atoiDefault(q.Get("page_size"), 20),
		Username: q.Get("username"),
	}
	if v, err := strconv.ParseInt(q.Get("user_id"), 10, 64); err == nil {
		req.UserID = v
	}
	if v, err := strconv.ParseInt(q.Get("problem_id"), 10, 64); err == nil {
		req.ProblemID = v
	}
	if v, err := strconv.Atoi(q.Get("status")); err == nil {
		req.Status = v
	}
	if v, err := strconv.ParseInt(q.Get("contest_id"), 10, 64); err == nil {
		req.ContestID = v
	}
	if v, err := strconv.ParseInt(q.Get("assignment_id"), 10, 64); err == nil {
		req.AssignmentID = v
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}
	where, args := []string{}, []any{}
	if req.Username != "" {
		where = append(where, "username=?")
		args = append(args, req.Username)
	}
	if req.UserID > 0 {
		where = append(where, "user_id=?")
		args = append(args, req.UserID)
	}
	if req.ProblemID > 0 {
		where = append(where, "problem_id=?")
		args = append(args, req.ProblemID)
	}
	if req.Status > 0 {
		where = append(where, "status=?")
		args = append(args, req.Status)
	}
	if req.ContestID > 0 {
		where = append(where, "contest_id=?")
		args = append(args, req.ContestID)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + stringsJoin(where, " AND ")
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM submissions `+cond, args...).Scan(&total); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := s.db.Query(`SELECT id, username, problem_id, problem_name, status, time_used, mem_used, created_at
		FROM submissions `+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, req.PageSize, (req.Page-1)*req.PageSize)...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []SubListItem{}
	for rows.Next() {
		var it SubListItem
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Username, &it.ProblemID, &it.ProblemName, &it.Status,
			&it.TimeUsed, &it.MemUsed, (*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		if t, ok := StatusText[it.Status]; ok {
			it.StatusText = t
		} else {
			it.StatusText = "等待中"
		}
		list = append(list, it)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": req.Page, "page_size": req.PageSize})
}

// submissionDetail 返回单条提交的完整代码与错误信息。
func (s *Server) submissionDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "提交编号无效")
		return
	}
	var sd SubListItem
	var created time.Time
	var code, errText string
	err := s.db.QueryRow(`SELECT id, username, problem_id, problem_name, status, time_used, mem_used,
		code, error, created_at FROM submissions WHERE id=?`, id).
		Scan(&sd.ID, &sd.Username, &sd.ProblemID, &sd.ProblemName, &sd.Status, &sd.TimeUsed, &sd.MemUsed,
			&code, &errText, (*time.Time)(&created))
	if err != nil {
		Fail(w, http.StatusNotFound, "提交不存在")
		return
	}
	if t, ok := StatusText[sd.Status]; ok {
		sd.StatusText = t
	}
	OK(w, map[string]any{"submission": sd, "code": code, "error": errText,
		"created_at": created.String()})
}

// stringsJoin 拼接字符串切片。
func stringsJoin(list []string, sep string) string {
	if len(list) == 0 {
		return ""
	}
	out := list[0]
	for _, v := range list[1:] {
		out += sep + v
	}
	return out
}

// statusCount 统计某用户名下各状态提交数。
func (s *Server) statusCount(status, userID int) int {
	var n int
	if userID > 0 {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE status=? AND user_id=?`, status, userID).Scan(&n)
		return n
	}
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE status=?`, status).Scan(&n)
	return n
}

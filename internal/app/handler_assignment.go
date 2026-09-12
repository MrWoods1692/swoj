package app

import (
	"net/http"
	"strconv"
	"time"
)

// AssignmentResp 作业列表与详情响应。
type AssignmentResp struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Info     string         `json:"info"`
	Visible  bool           `json:"visible"`
	Deadline string         `json:"deadline"`
	State    string         `json:"state"`
	Problems []ProblemBrief `json:"problems"`
}

// AssignmentCreateReq 作业创建/编辑请求，仅管理员可用。
type AssignmentCreateReq struct {
	Name      string `json:"name"`
	Info      string `json:"info"`
	Deadline  string `json:"deadline"`
	Visible   bool   `json:"visible"`
	ProblemID int64  `json:"problem_id"`
}

// assignmentState 由截止时间推导作业阶段。
func assignmentState(deadline, now time.Time) string {
	if now.After(deadline) {
		return "已截止"
	}
	return "进行中"
}

// assignmentList 返回全部可见作业及其阶段。
func (s *Server) assignmentList(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	rows, err := s.db.Query(`SELECT id, name, info, visible, deadline FROM assignments
		WHERE visible=1 ORDER BY id DESC`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []AssignmentResp{}
	for rows.Next() {
		var a AssignmentResp
		var deadline time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.Info, &a.Visible, (*time.Time)(&deadline)); err != nil {
			continue
		}
		a.Deadline = deadline.String()
		a.State = assignmentState(deadline, now)
		a.Problems = []ProblemBrief{}
		list = append(list, a)
	}
	OK(w, list)
}

// assignmentDetail 返回单个作业与其题目清单。
func (s *Server) assignmentDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "作业编号无效")
		return
	}
	var a AssignmentResp
	var deadline time.Time
	err := s.db.QueryRow(`SELECT id, name, info, visible, deadline FROM assignments WHERE id=?`, id).
		Scan(&a.ID, &a.Name, &a.Info, &a.Visible, (*time.Time)(&deadline))
	if err != nil {
		Fail(w, http.StatusNotFound, "作业不存在")
		return
	}
	a.Deadline = deadline.String()
	a.State = assignmentState(deadline, time.Now())
	probs, err := briefProblems(s.db,
		`SELECT p.id, p.name, p.difficulty, ap.order_no, p.accept, p.submit
		 FROM assignment_problems ap JOIN problems p ON p.id=ap.problem_id
		 WHERE ap.assignment_id=? ORDER BY ap.order_no, p.id`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.Problems = probs
	OK(w, a)
}

// assignmentCreate 管理员创建作业并挂载一道题目。
func (s *Server) assignmentCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if !requireAdmin(w, claims) {
		return
	}
	var req AssignmentCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	deadline, err := time.ParseInLocation("2006-01-02 15:04:05", req.Deadline, time.Local)
	if err != nil {
		Fail(w, http.StatusBadRequest, "截止时间格式错误，应为 YYYY-MM-DD HH:MM:SS")
		return
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM problems WHERE id=?`, req.ProblemID).Scan(&n)
	if n == 0 {
		Fail(w, http.StatusNotFound, "题目不存在")
		return
	}
	res, err := s.db.Exec(`INSERT INTO assignments(name, info, visible, deadline, creator)
		VALUES(?,?,?,?,?)`, req.Name, req.Info, req.Visible, deadline, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	aid, _ := res.LastInsertId()
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO assignment_problems
		(assignment_id, problem_id, order_no) VALUES(?,?,1)`, aid, req.ProblemID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "assignment_create", strconv.FormatInt(aid, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": aid})
}

// wrongQuestionList 返回当前用户的错题本，仅显示仍未通过且未删除的记录。
func (s *Server) wrongQuestionList(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, problem_id, problem_name, times, last_try_at FROM wrong_questions
		WHERE user_id=? AND accepted=0 AND removed=0 ORDER BY last_try_at DESC, id DESC LIMIT 200`,
		claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []WrongQuestionResp{}
	for rows.Next() {
		var q WrongQuestionResp
		var last time.Time
		if err := rows.Scan(&q.ID, &q.ProblemID, &q.ProblemName, &q.Times, (*time.Time)(&last)); err != nil {
			continue
		}
		q.LastTryAt = last.String()
		list = append(list, q)
	}
	OK(w, list)
}

// wrongQuestionRemove 从错题本移除一条记录（软删除，不删除真实提交）。
func (s *Server) wrongQuestionRemove(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "编号无效")
		return
	}
	res, err := s.db.Exec(`UPDATE wrong_questions SET removed=1
		WHERE id=? AND user_id=? AND removed=0`, id, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "错题记录不存在")
		return
	}
	OK(w, map[string]any{"removed": true})
}

// WrongQuestionResp 错题记录响应。
type WrongQuestionResp struct {
	ID          int64  `json:"id"`
	ProblemID   int64  `json:"problem_id"`
	ProblemName string `json:"problem_name"`
	Times       int    `json:"times"`
	LastTryAt   string `json:"last_try_at"`
}

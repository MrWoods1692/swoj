package app

import (
	"net/http"
	"strconv"
	"strings"
)

// SubmitReq 提交请求。
type SubmitReq struct {
	ProblemID    int64  `json:"problem_id"`
	Code         string `json:"code"`
	Mode         string `json:"mode"`
	ContestID    int64  `json:"contest_id"`
	AssignmentID int64  `json:"assignment_id"`
}

// submit 校验权限后把提交写入数据库并投递到测评队列。
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r)
	if !ok {
		Fail(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var req SubmitReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Mode == "" {
		req.Mode = "cpp"
	}
	if req.Mode != "cpp" {
		Fail(w, http.StatusBadRequest, "当前仅支持 C++ 提交")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		Fail(w, http.StatusBadRequest, "代码不能为空")
		return
	}
	if len(req.Code) > 512*1024 {
		Fail(w, http.StatusBadRequest, "代码过长")
		return
	}

	var u User
	row := s.db.QueryRow(`SELECT id, username, can_submit FROM users WHERE id=?`, claims.UserID)
	if err := row.Scan(&u.ID, &u.Username, &u.CanSubmit); err != nil {
		Fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	if !u.CanSubmit {
		Fail(w, http.StatusForbidden, "该账号已被禁止提交")
		return
	}

	var p Problem
	if err := s.db.QueryRow(`SELECT id, name, time_limit, mem_limit FROM problems WHERE id=?`, req.ProblemID).
		Scan(&p.ID, &p.Name, &p.TimeLimit, &p.MemLimit); err != nil {
		Fail(w, http.StatusNotFound, "题目不存在")
		return
	}

	res, err := s.db.Exec(`INSERT INTO submissions(user_id,username,problem_id,problem_name,code,mode,contest_id,
		assignment_id,status) VALUES(?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Username, p.ID, p.Name, req.Code, req.Mode, req.ContestID, req.AssignmentID, StatusPending)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	subID, _ := res.LastInsertId()

	if err := s.Queue.Enqueue(JudgeTask{SubID: subID, UserID: u.ID, ProblemID: p.ID, Code: req.Code}); err != nil {
		Fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	logOp(s.db, claims, "submit", strconv.FormatInt(p.ID, 10), "提交题解", clientIP(r))
	OK(w, map[string]any{"id": subID, "status": StatusPending})
}

// SubmissionResp 提交记录响应。
type SubmissionResp struct {
	ID           int64   `json:"id"`
	Username     string  `json:"username"`
	ProblemID    int64   `json:"problem_id"`
	ProblemName  string  `json:"problem_name"`
	Mode         string  `json:"mode"`
	Status       int     `json:"status"`
	StatusText   string  `json:"status_text"`
	TimeUsed     int     `json:"time_used"`
	MemUsed      int     `json:"mem_used"`
	ContestID    int64   `json:"contest_id"`
	AssignmentID int64   `json:"assignment_id"`
	CreatedAt    string  `json:"created_at"`
}

// toResp 把提交行转成响应结构。
func toResp(sub Submission) SubmissionResp {
	st := SubmissionResp{
		ID: sub.ID, Username: sub.Username, ProblemID: sub.ProblemID, ProblemName: sub.ProblemName,
		Mode: sub.Mode, Status: sub.Status, TimeUsed: sub.TimeUsed, MemUsed: sub.MemUsed,
		ContestID: sub.ContestID, AssignmentID: sub.AssignmentID, CreatedAt: sub.CreatedAt.String(),
	}
	if t, ok := StatusText[sub.Status]; ok {
		st.StatusText = t
	} else {
		st.StatusText = "等待中"
	}
	return st
}

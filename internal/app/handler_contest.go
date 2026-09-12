package app

import (
	"net/http"
	"strconv"
	"time"
)

// ContestResp 比赛列表与详情响应。
type ContestResp struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Info      string         `json:"info"`
	Visible   bool           `json:"visible"`
	Rank      bool           `json:"rank"`
	StartTime string         `json:"start_time"`
	EndTime   string         `json:"end_time"`
	State     string         `json:"state"`
	Submit    int            `json:"submit"`
	Accept    int            `json:"accept"`
	Enrolled  bool           `json:"enrolled"`
	Problems  []ProblemBrief `json:"problems"`
}

// RankEntry 排行榜单行。
type RankEntry struct {
	Username string `json:"username"`
	Accepted int    `json:"accepted"`
	FirstAt  string `json:"first_at"`
}

// contestState 由当前时间与起止时间推导比赛阶段。
func contestState(start, end, now time.Time) string {
	switch {
	case now.Before(start):
		return "未开始"
	case now.After(end):
		return "已结束"
	}
	return "进行中"
}

// loadContest 读取单个比赛；密码用于门禁判断但不进入响应。
func (s *Server) loadContest(id int64) (ContestResp, string, bool) {
	var c ContestResp
	var pw string
	var start, end time.Time
	err := s.db.QueryRow(`SELECT id, name, info, visible, rank, start_time, end_time, accept, submit, password
		FROM contests WHERE id=?`, id).
		Scan(&c.ID, &c.Name, &c.Info, &c.Visible, &c.Rank,
			(*time.Time)(&start), (*time.Time)(&end), &c.Accept, &c.Submit, &pw)
	if err != nil {
		return ContestResp{}, "", false
	}
	c.StartTime = start.String()
	c.EndTime = end.String()
	c.State = contestState(start, end, time.Now())
	c.Problems = []ProblemBrief{}
	return c, pw, true
}

// contestList 返回全部可见比赛，附带进行阶段与当前用户报名状态。
func (s *Server) contestList(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	enrolled := map[int64]bool{}
	if uid := viewerID(r); uid > 0 {
		rs, err := s.db.Query(`SELECT contest_id FROM contest_users WHERE user_id=?`, uid)
		if err == nil {
			for rs.Next() {
				var id int64
				if rs.Scan(&id) == nil {
					enrolled[id] = true
				}
			}
			rs.Close()
		}
	}

	rows, err := s.db.Query(`SELECT id, name, info, visible, rank, start_time, end_time, accept, submit
		FROM contests WHERE visible=1 ORDER BY rank DESC, id`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ContestResp{}
	for rows.Next() {
		var c ContestResp
		var start, end time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Info, &c.Visible, &c.Rank,
			(*time.Time)(&start), (*time.Time)(&end), &c.Accept, &c.Submit); err != nil {
			continue
		}
		c.StartTime = start.String()
		c.EndTime = end.String()
		c.State = contestState(start, end, now)
		c.Enrolled = enrolled[c.ID]
		list = append(list, c)
	}
	OK(w, list)
}

// contestDetail 返回比赛详情与其题目清单；私有比赛要求已报名。
func (s *Server) contestDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	c, pw, ok := s.loadContest(id)
	if !ok {
		Fail(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if pw != "" {
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM contest_users WHERE contest_id=? AND user_id=?`,
			id, viewerID(r)).Scan(&n)
		if n == 0 {
			Fail(w, http.StatusForbidden, "该比赛为私有比赛，请先报名")
			return
		}
		c.Enrolled = true
	}
	probs, err := briefProblems(s.db,
		`SELECT p.id, p.name, p.difficulty, cp.order_no, p.accept, p.submit
		 FROM contest_problems cp JOIN problems p ON p.id=cp.problem_id
		 WHERE cp.contest_id=? ORDER BY cp.order_no, p.id`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	c.Problems = probs
	OK(w, c)
}

// ContestCreateReq 管理员创建比赛的请求体。
type ContestCreateReq struct {
	Name       string  `json:"name"`
	Info       string  `json:"info"`
	Visible    bool    `json:"visible"`
	Rank       bool    `json:"rank"`
	Password   string  `json:"password"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	ProblemIDs []int64 `json:"problem_ids"`
}

// contestCreate 管理员创建比赛并挂载题目。
func (s *Server) contestCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if !requireAdmin(w, claims) {
		return
	}
	var req ContestCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, time.Local)
	if err != nil {
		Fail(w, http.StatusBadRequest, "开始时间格式错误，应为 YYYY-MM-DD HH:MM:SS")
		return
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, time.Local)
	if err != nil {
		Fail(w, http.StatusBadRequest, "结束时间格式错误，应为 YYYY-MM-DD HH:MM:SS")
		return
	}
	if !end.After(start) {
		Fail(w, http.StatusBadRequest, "结束时间必须晚于开始时间")
		return
	}
	res, err := s.db.Exec(`INSERT INTO contests(name, info, rank, visible, password, start_time, end_time, creator)
		VALUES(?,?,?,?,?,?,?,?)`, req.Name, req.Info, req.Rank, req.Visible, req.Password, start, end, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	cid, _ := res.LastInsertId()
	for i, pid := range req.ProblemIDs {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM problems WHERE id=?`, pid).Scan(&n); err != nil || n == 0 {
			continue
		}
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO contest_problems(contest_id, problem_id, order_no) VALUES(?,?,?)`,
			cid, pid, i+1)
	}
	logOp(s.db, claims, "contest_create", strconv.FormatInt(cid, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": cid})
}

// contestEnroll 报名比赛，写入报名关系表。
func (s *Server) contestEnroll(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM contests WHERE id=?`, id).Scan(&n); err != nil || n == 0 {
		Fail(w, http.StatusNotFound, "比赛不存在")
		return
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO contest_users(contest_id, user_id) VALUES(?,?)`,
		id, claims.UserID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "contest_enroll", strconv.FormatInt(id, 10), "报名比赛", clientIP(r))
	OK(w, map[string]any{"enrolled": true})
}

// contestRank 返回比赛内的个人通过排行榜，按通过题数降序、首次通过时间升序。
//
// created_at 存的是 SQLite CURRENT_TIMESTAMP 格式（"YYYY-MM-DD HH:MM:SS"，无时区），
// modernc.org/sqlite 不会隐式解析成 time.Time——这里直接 Scan 成 string，
// 排序与展示都用原始字符串（ISO 前缀顺序 = 时间升序）。
func (s *Server) contestRank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	rows, err := s.db.Query(`SELECT username, COUNT(DISTINCT problem_id),
		COALESCE(MIN(created_at), '') AS first_at
		FROM submissions WHERE contest_id=? AND status=? GROUP BY username
		ORDER BY COUNT(DISTINCT problem_id) DESC, MIN(created_at) ASC LIMIT 100`, id, StatusAccepted)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []RankEntry{}
	for rows.Next() {
		var e RankEntry
		var acc int
		if err := rows.Scan(&e.Username, &acc, &e.FirstAt); err != nil {
			continue
		}
		e.Accepted = acc
		list = append(list, e)
	}
	OK(w, list)
}

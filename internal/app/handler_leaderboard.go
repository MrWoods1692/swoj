package app

import (
	"net/http"
	"strconv"
	"time"
)

// LeaderRank 排行榜单行：按 AC 数降序、总用时升序。
type LeaderRank struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	Accepted  int    `json:"accepted"`
	TimeUsed  int    `json:"time_used"`
	Submitted int    `json:"submitted"`
	School    string `json:"school"`
	Rate      string `json:"rate"`
}

// LeaderboardReq 排行榜查询参数。
type LeaderboardReq struct {
	ProblemID int64  `json:"problem_id"`
	ContestID int64  `json:"contest_id"`
	Username  string `json:"username"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}

// leaderboard 返回全站或按题目/比赛的排行榜。
func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := LeaderboardReq{Page: atoiDefault(q.Get("page"), 1), PageSize: atoiDefault(q.Get("page_size"), 50),
		Username: q.Get("username")}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 50
	}

	where, args := []string{}, []any{}
	if req.ProblemID > 0 {
		where = append(where, "s.problem_id=?")
		args = append(args, req.ProblemID)
	}
	if req.ContestID > 0 {
		where = append(where, "s.contest_id=?")
		args = append(args, req.ContestID)
	}
	if req.Username != "" {
		where = append(where, "s.username LIKE ?")
		args = append(args, "%"+req.Username+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + stringsJoin(where, " AND ")
	}

	// 总表：每个用户的总提交数，用于计算通过率。
	totalBy := map[string]int{}
	trs, err := s.db.Query(`SELECT username, COUNT(*) FROM submissions GROUP BY username`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	for trs.Next() {
		var un string
		var c int
		if trs.Scan(&un, &c) == nil {
			totalBy[un] = c
		}
	}
	trs.Close()

	// AC 数为主排序键，通过的提交总用时为次排序键。
	rows, err := s.db.Query(`SELECT s.user_id, s.username, COUNT(DISTINCT CASE WHEN s.status=? THEN s.problem_id END),
		COALESCE(SUM(CASE WHEN s.status=? THEN s.time_used ELSE 0 END),0)
		FROM submissions s `+cond+`
		GROUP BY s.user_id ORDER BY COUNT(DISTINCT CASE WHEN s.status=? THEN s.problem_id END) DESC,
			SUM(CASE WHEN s.status=? THEN s.time_used ELSE 0 END) ASC LIMIT ? OFFSET ?`,
		append(append(args, StatusAccepted, StatusAccepted), StatusAccepted, StatusAccepted,
			req.PageSize, (req.Page-1)*req.PageSize)...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	list := []LeaderRank{}
	rank := (req.Page - 1) * req.PageSize
	for rows.Next() {
		var lr LeaderRank
		var acc, tUsed int
		if err := rows.Scan(&lr.UserID, &lr.Username, &acc, &tUsed); err != nil {
			continue
		}
		lr.Accepted, lr.TimeUsed = acc, tUsed
		rank++
		lr.Rank = rank
		lr.Submitted = totalBy[lr.Username]
		if lr.Submitted == 0 {
			lr.Submitted = acc
		}
		lr.Rate = fmtPct(acc, lr.Submitted)
		list = append(list, lr)
	}
	OK(w, map[string]any{"list": list, "page": req.Page, "page_size": req.PageSize})
}

// myStats 返回当前用户的个人统计概览。
func (s *Server) myStats(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	uid := claims.UserID
	var submitted, accepted, wrong int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=?`, uid).Scan(&submitted)
	_ = s.db.QueryRow(`SELECT COUNT(DISTINCT problem_id) FROM submissions WHERE user_id=? AND status=?`,
		uid, StatusAccepted).Scan(&accepted)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM wrong_questions WHERE user_id=? AND removed=0`, uid).Scan(&wrong)

	// 完成率基准为题库中的可见题目总数，而不是已有人 AC 的题数。
	var totalProblems int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM problems WHERE invisible=0`).Scan(&totalProblems)
	if totalProblems == 0 {
		_ = s.db.QueryRow(`SELECT COUNT(DISTINCT problem_id) FROM submissions WHERE status=?`, StatusAccepted).Scan(&totalProblems)
	}

	// 名次：通过题数严格多于当前用户的人数 + 1。
	var rankNo int
	_ = s.db.QueryRow(`SELECT COUNT(*)+1 FROM (SELECT username, COUNT(DISTINCT problem_id) AS a
		FROM submissions WHERE status=? GROUP BY username) x WHERE x.a > ?`,
		StatusAccepted, accepted).Scan(&rankNo)

	OK(w, map[string]any{
		"submitted": submitted, "accepted": accepted, "wrong": wrong,
		"total_problems": totalProblems, "rank": rankNo,
		"solved_rate": fmtPct(accepted, totalProblems),
	})
}

// fmtPct 计算百分比字符串。
func fmtPct(num, den int) string {
	if den <= 0 {
		return "0.00%"
	}
	return strconv.FormatFloat(float64(num)/float64(den)*100, 'f', 2, 64) + "%"
}

// AIAskReq AI 问答请求。
type AIAskReq struct {
	ProblemID int64  `json:"problem_id"`
	Question  string `json:"question"`
	Code      string `json:"code"`
	Status    int    `json:"status"`
}

// aiAsk 调用外部大模型问答；未配置 APIKey 时返回可读提示而非报错。
func (s *Server) aiAsk(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if !s.AI.Enabled {
		Fail(w, http.StatusServiceUnavailable, "AI 服务未启用，请先在服务端配置 SWOJ_AI_ENABLED 与 SWOJ_AI_KEY")
		return
	}
	var req AIAskReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Question == "" {
		Fail(w, http.StatusBadRequest, "问题不能为空")
		return
	}
	answer, src, err := callAI(s.AI, s.aiPrompt(req))
	if err != nil {
		Fail(w, http.StatusBadGateway, "AI 服务调用失败："+err.Error())
		return
	}
	res, err := s.db.Exec(`INSERT INTO ai_qas(user_id, problem_id, question, answer, source) VALUES(?,?,?,?,?)`,
		claims.UserID, req.ProblemID, req.Question, answer, src)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	OK(w, map[string]any{"id": id, "answer": answer, "source": src})
}

// aiPrompt 拼装发给模型的上下文，含题目信息、代码与当前判题状态。
func (s *Server) aiPrompt(req AIAskReq) string {
	ctx := ""
	if req.ProblemID > 0 {
		var name, content string
		if err := s.db.QueryRow(`SELECT name, content FROM problems WHERE id=?`, req.ProblemID).
			Scan(&name, &content); err == nil {
			ctx += "\n题目：" + name + "\n题目内容：" + content
		}
	}
	if t, ok := StatusText[req.Status]; ok {
		ctx += "\n当前判题状态：" + t
	}
	if req.Code != "" {
		if len(req.Code) > 12000 {
			req.Code = req.Code[:12000]
		}
		ctx += "\n用户代码：\n" + req.Code
	}
	return "你是一个算法竞赛辅导助手。请结合以下上下文，用中文给出简洁、可执行的分析与建议。" +
		ctx + "\n\n用户问题：" + req.Question
}

// aiHistory 返回当前用户最近的 AI 问答记录。
func (s *Server) aiHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, problem_id, question, answer, source, created_at FROM ai_qas
		WHERE user_id=? ORDER BY id DESC LIMIT 50`, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []AIHistoryItem{}
	for rows.Next() {
		var it AIHistoryItem
		var created time.Time
		if err := rows.Scan(&it.ID, &it.ProblemID, &it.Question, &it.Answer, &it.Source,
			(*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		list = append(list, it)
	}
	OK(w, list)
}

// AIHistoryItem AI 问答历史记录。
type AIHistoryItem struct {
	ID        int64  `json:"id"`
	ProblemID int64  `json:"problem_id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

package app

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
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

// AIAskReq AI 问答请求（JSON 版本）。
type AIAskReq struct {
	ProblemID int64  `json:"problem_id"`
	Question  string `json:"question"`
	Code      string `json:"code"`
	Status    int    `json:"status"`
}

// AIFileItem 描述 multipart 上传的单个文件字段。
type AIFileItem struct {
	Name    string
	Content string
}

// aiAsk 调用外部大模型问答；支持 JSON 与 multipart 两种上传方式。
// multipart 上传时字段名：
//
//	problem_id / status  可选
//	question            可选，纯文本提问
//	code                可选，代码文本
//	files[]             可选，附件，允许多个；支持 .cpp/.txt/.in/.out 等文本文件
//
// 附件内容会被拼接进 prompt，与 code/question 一同发送给模型。
func (s *Server) aiAsk(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	if !s.AI.Enabled {
		Fail(w, http.StatusServiceUnavailable, "AI 服务未启用，请管理员在后台配置 AI Token")
		return
	}

	var req AIAskReq
	var files []AIFileItem

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		// 上传限额：单文件 1MB，总数 5 个，总大小 3MB；超出直接 413。
		if err := r.ParseMultipartForm(3 << 20); err != nil {
			Fail(w, http.StatusRequestEntityTooLarge, "上传过大："+err.Error())
			return
		}
		val := func(k string) string { v := r.FormValue(k); return v }
		if v := val("problem_id"); v != "" {
			req.ProblemID = int64(atoiDefault(v, 0))
		}
		if v := val("status"); v != "" {
			req.Status = atoiDefault(v, 0)
		}
		req.Question = val("question")
		req.Code = val("code")

		hf, ok := r.MultipartForm.File["files[]"]
		if !ok {
			hf = r.MultipartForm.File["files"]
		}
		if ok {
			for i, fh := range hf {
				if i >= 5 {
					Fail(w, http.StatusBadRequest, "最多上传 5 个文件")
					return
				}
				if fh.Size > 1<<20 {
					Fail(w, http.StatusBadRequest, "单个文件不能超过 1MB")
					return
				}
				f, err := fh.Open()
				if err != nil {
					Fail(w, http.StatusInternalServerError, err.Error())
					return
				}
				buf, err := io.ReadAll(io.LimitReader(f, 1<<20))
				_ = f.Close()
				if err != nil {
					Fail(w, http.StatusInternalServerError, err.Error())
					return
				}
				if !isTextFile(fh.Filename) {
					Fail(w, http.StatusBadRequest,
						"文件类型不支持："+fh.Filename+"（仅支持 .cpp/.txt/.in/.out/.c/.h/.py/.md 等文本文件）")
					return
				}
				files = append(files, AIFileItem{Name: fh.Filename, Content: string(buf)})
			}
		}
	} else {
		if err := decode(r, &req); err != nil {
			Fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	question := strings.TrimSpace(req.Question)
	// 有附件但没有 question 时，把文件内容作为提问主体；否则文件只作为上下文。
	if question == "" && len(files) == 0 && req.Code == "" {
		Fail(w, http.StatusBadRequest, "问题或附件不能全为空")
		return
	}

	// 所有请求体校验通过后再检查 AI 服务可用性；否则文件校验会被 503 掩盖。
	provider := strings.ToLower(strings.TrimSpace(s.AI.Provider))
	if provider == "yunzhi" && s.AI.YunzhiToken == "" {
		Fail(w, http.StatusServiceUnavailable, "AI 服务未启用，请管理员在后台配置 AI Token")
		return
	}
	if provider != "yunzhi" && provider != "" && s.AI.APIKey == "" {
		Fail(w, http.StatusServiceUnavailable, "AI 服务未启用，请管理员在后台配置 API Key")
		return
	}

	answer, src, err := callAI(s.AI, s.aiPrompt(req, files))
	if err != nil {
		Fail(w, http.StatusBadGateway, "AI 服务调用失败："+err.Error())
		return
	}

	// 保存到 ai_qas：question 存用户提问；若无 question 但有文件，落一份文件摘要。
	stored := question
	if stored == "" && len(files) > 0 {
		stored = "(文件附件：" + strings.Join(fileNames(files), ", ") + ")"
	}

	res, err := s.db.Exec(`INSERT INTO ai_qas(user_id, problem_id, question, answer, source) VALUES(?,?,?,?,?)`,
		claims.UserID, req.ProblemID, stored, answer, src)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	OK(w, map[string]any{"id": id, "answer": answer, "source": src})
}

// aiPrompt 拼装发给模型的上下文，含题目信息、代码、当前判题状态与附件内容。
func (s *Server) aiPrompt(req AIAskReq, files []AIFileItem) string {
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
	for _, f := range files {
		content := f.Content
		if len(content) > 12000 {
			content = content[:12000] + "\n…(已截断)"
		}
		ctx += "\n附件 " + f.Name + " 内容：\n" + content
	}
	q := strings.TrimSpace(req.Question)
	if q == "" {
		q = "请分析以上代码/文件内容，指出问题并给出改进方案。"
	}
	return "用户问题：" + q + ctx
}

// isTextFile 判断扩展名是否是可发送给模型的文本文件。
func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".cpp", ".cxx", ".cc", ".c", ".h", ".hpp", ".hxx",
		".txt", ".in", ".out", ".md", ".py", ".java", ".js", ".ts", ".go",
		".rs", ".rb", ".sh", ".sql", ".json", ".xml", ".yml", ".yaml", ".csv", ".log":
		return true
	}
	return false
}

// fileNames 提取附件文件名列表。
func fileNames(files []AIFileItem) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}

// aiHistory 返回当前用户最近的 AI 问答记录；只保留近 7 天内的记录。
func (s *Server) aiHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	since := time.Now().Add(-aiRetentionDuration)
	rows, err := s.db.Query(`SELECT id, problem_id, question, answer, source, created_at FROM ai_qas
		WHERE user_id=? AND created_at >= ? ORDER BY id DESC LIMIT 50`, claims.UserID, since)
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

// AICfgReq 管理员后台保存 AI 配置的请求体。
type AICfgReq struct {
	Token        string `json:"token"`
	Provider     string `json:"provider"`
	SystemPrompt string `json:"system_prompt"`
	ClearToken   bool   `json:"clear_token"` // 显式清除已保存的 Token
}

// aiConfigGet 返回当前 AI 配置（token 做掩码处理）。
// 非空 token 显示为前 4 + **** + 后 4；未配置时为空串。
func (s *Server) aiConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	// 每次从 DB 重新读取一次，保证返回的是后台最新值。
	loadAIConfig(s.db, s.AI)
	OK(w, map[string]any{
		"enabled":       s.AI.Enabled,
		"provider":      s.AI.Provider,
		"yunzhi_url":    s.AI.YunzhiURL,
		"token_masked":  maskToken(s.AI.YunzhiToken),
		"has_token":     s.AI.YunzhiToken != "",
		"system_prompt": s.AI.SystemPrompt,
		"default_prompt": DefaultAISystemPrompt,
		"retention_days": aiRetentionDays,
	})
}

// aiConfigSave 保存 AI 配置。空 token 表示清除；system_prompt 空表示恢复默认。
func (s *Server) aiConfigSave(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	var req AICfgReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "yunzhi"
	}
	if provider != "yunzhi" && provider != "openai" {
		Fail(w, http.StatusBadRequest, "provider 仅支持 yunzhi 或 openai")
		return
	}
	token := strings.TrimSpace(req.Token)
	if token != "" && len(token) > 256 {
		Fail(w, http.StatusBadRequest, "token 过长")
		return
	}
	sys := req.SystemPrompt
	if len(sys) > 4000 {
		Fail(w, http.StatusBadRequest, "system_prompt 不能超过 4000 字")
		return
	}
	clearToken := req.ClearToken

	// 保存配置：空 token 视为「不修改」，避免管理员只想更新提示词却误清 Token。
	// 需要显式清除 Token 时，管理员可传 clear_token=true。
	if token != "" {
		_, _ = s.db.Exec(`INSERT INTO admin_configs(key, value) VALUES('ai.token', ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, token)
		s.AI.YunzhiToken = token
		s.AI.Enabled = true
	} else if clearToken {
		_, _ = s.db.Exec(`UPDATE admin_configs SET value='' WHERE key='ai.token'`)
		s.AI.YunzhiToken = ""
	}

	_, _ = s.db.Exec(`INSERT INTO admin_configs(key, value) VALUES('ai.provider', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, provider)
	_, _ = s.db.Exec(`INSERT INTO admin_configs(key, value) VALUES('ai.system_prompt', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, sys)

	// 立即回写到内存
	s.AI.Provider = provider
	s.AI.SystemPrompt = sys

	OK(w, map[string]any{
		"provider":     provider,
		"token_masked": maskToken(s.AI.YunzhiToken),
		"has_token":    s.AI.YunzhiToken != "",
	})
}

// maskToken 生成 token 的展示形式：短于 12 位时全部打星；否则前 4 + **** + 后 4。
func maskToken(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if len(t) < 12 {
		return strings.Repeat("*", len(t))
	}
	return t[:4] + "****" + t[len(t)-4:]
}

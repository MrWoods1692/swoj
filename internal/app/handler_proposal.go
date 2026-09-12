package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 学生出题：用户提交题目草稿，进入待审核状态；老师或管理员审核通过后，
// 草稿转换为 problems/cases 进入题库，并按默认或指定分值给用户发放积分。
// 拒绝的草稿留档，便于用户修改后重新提交。
//
// 与「资料」「笔记」不同：这里存在真实的用户内容到全局题库的写入路径，
// 因此拒绝与审核动作都要写 operation_logs，避免题库内容来源不可追溯。

// proposalStatus 状态取值白名单。
const (
	PPStatusPending   = "pending"
	PPStatusApproved  = "approved"
	PPStatusRejected  = "rejected"
	PPStatusWithdrawn = "withdrawn" // 作者主动撤回，等待重新提交
)

// ppRewardDefault 审核通过时的默认奖励积分，与积分规则的 CategoryAdmin 语义一致。
const ppRewardDefault = 10

// ProposalCaseReq 提议中一组样例。
type ProposalCaseReq struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// ProposalCreateReq 学生新建出题请求。
type ProposalCreateReq struct {
	Name         string            `json:"name"`
	Background   string            `json:"background"`
	Description  string            `json:"description"`
	InputFormat  string            `json:"input_format"`
	OutputFormat string            `json:"output_format"`
	Hint         string            `json:"hint"`
	Cases        []ProposalCaseReq `json:"cases"`
}

// ProposalResp 提议对外结构。
type ProposalResp struct {
	ID          int64              `json:"id"`
	AuthorID    int64              `json:"author_id"`
	AuthorName  string             `json:"author_name"`
	Name        string             `json:"name"`
	Background  string             `json:"background"`
	Description string             `json:"description"`
	InputFormat string             `json:"input_format"`
	OutputFormat string            `json:"output_format"`
	Hint        string             `json:"hint"`
	CaseCount   int                `json:"case_count"`
	Status      string             `json:"status"`
	ReviewComment string             `json:"review_comment"`
	ReviewerID    int64              `json:"reviewer_id"`
	ReviewedAt    string             `json:"reviewed_at"`
	RewardPoints  int                `json:"reward_points"`
	ApprovedProblemID int64          `json:"approved_problem_id"`
	CreatedAt     string             `json:"created_at"`
}

// proposalByID 读取单条提议（含样例计数）。
func (s *Server) proposalByID(id int64) (*ProposalResp, bool) {
	var authorID int64
	var authorName, name, background, description, inputFormat, outputFormat, hint string
	var status, reviewComment, reviewedAt string
	var reviewerID, approvedProblemID int64
	var rewardPoints int
	var created time.Time
	err := s.db.QueryRow(`SELECT id, author_id, author_name, name, background, description,
		input_format, output_format, hint, status, review_comment, reviewer_id,
		reward_points, approved_problem_id, created_at
		FROM problem_proposals WHERE id=?`, id).
		Scan(&id, &authorID, &authorName, &name, &background, &description,
			&inputFormat, &outputFormat, &hint, &status, &reviewComment, &reviewerID,
			&rewardPoints, &approvedProblemID, (*time.Time)(&created))
	if err != nil {
		return nil, false
	}
	caseCount := 0
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM proposal_cases WHERE proposal_id=?`, id).Scan(&caseCount)
	return &ProposalResp{
		ID: id, AuthorID: authorID, AuthorName: authorName, Name: name,
		Background: background, Description: description,
		InputFormat: inputFormat, OutputFormat: outputFormat, Hint: hint,
		CaseCount: caseCount, Status: status,
		ReviewComment: reviewComment, ReviewerID: reviewerID,
		ReviewedAt: reviewedAt, RewardPoints: rewardPoints,
		ApprovedProblemID: approvedProblemID,
		CreatedAt: created.Format(time.RFC3339),
	}, true
}

// proposalDetailFull 返回提议 + 完整样例（列表接口不带样例，节省带宽）。
func (s *Server) proposalDetailFull(id int64) (map[string]any, bool) {
	p, ok := s.proposalByID(id)
	if !ok {
		return nil, false
	}
	rows, err := s.db.Query(`SELECT index_no, input, output FROM proposal_cases WHERE proposal_id=? ORDER BY index_no`, id)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	cases := []ProblemCase{}
	for rows.Next() {
		var idx int
		var c ProblemCase
		if err := rows.Scan(&idx, &c.Input, &c.Output); err == nil {
			cases = append(cases, c)
		}
	}
	return map[string]any{
		"id": p.ID, "author_id": p.AuthorID, "author_name": p.AuthorName,
		"name": p.Name, "background": p.Background, "description": p.Description,
		"input_format": p.InputFormat, "output_format": p.OutputFormat,
		"hint": p.Hint, "cases": cases, "case_count": p.CaseCount,
		"status": p.Status, "review_comment": p.ReviewComment,
		"reviewer_id": p.ReviewerID, "reviewed_at": p.ReviewedAt,
		"reward_points": p.RewardPoints,
		"approved_problem_id": p.ApprovedProblemID,
		"created_at": p.CreatedAt,
	}, true
}

// proposalValidate 校验提议字段：名称/描述必填；用例至少 1 组且每组的输入/输出不能都空；
// 单字段长度受限；用例最多 50 组。
func proposalValidate(req *ProposalCreateReq) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Background = strings.TrimSpace(req.Background)
	req.Description = strings.TrimSpace(req.Description)
	req.InputFormat = strings.TrimSpace(req.InputFormat)
	req.OutputFormat = strings.TrimSpace(req.OutputFormat)
	req.Hint = strings.TrimSpace(req.Hint)
	if req.Name == "" || len([]rune(req.Name)) > 80 {
		return ErrProposalInvalidField("题目名称不能为空且不超过 80 字")
	}
	if req.Description == "" || len([]rune(req.Description)) > 50_000 {
		return ErrProposalInvalidField("题目描述不能为空且不超过 5 万字")
	}
	if len([]rune(req.Background)) > 10_000 {
		return ErrProposalInvalidField("题目背景不能超过 1 万字")
	}
	if len([]rune(req.InputFormat)) > 5_000 {
		return ErrProposalInvalidField("输入格式不能超过 5 千字")
	}
	if len([]rune(req.OutputFormat)) > 5_000 {
		return ErrProposalInvalidField("输出格式不能超过 5 千字")
	}
	if len([]rune(req.Hint)) > 5_000 {
		return ErrProposalInvalidField("提示不能超过 5 千字")
	}
	if len(req.Cases) == 0 {
		return ErrProposalInvalidField("至少需要一组样例")
	}
	if len(req.Cases) > 50 {
		return ErrProposalInvalidField("样例最多 50 组")
	}
	for i, c := range req.Cases {
		c.Input = strings.TrimSpace(c.Input)
		c.Output = strings.TrimSpace(c.Output)
		if c.Input == "" && c.Output == "" {
			req.Cases[i] = c
			continue
		}
		if len([]rune(c.Input)) > 10_000 {
			return ErrProposalInvalidField("样例输入过长")
		}
		if len([]rune(c.Output)) > 10_000 {
			return ErrProposalInvalidField("样例输出过长")
		}
		req.Cases[i] = c
	}
	// 至少保留 1 组非空样例
	hasNonEmpty := false
	for _, c := range req.Cases {
		if strings.TrimSpace(c.Input) != "" || strings.TrimSpace(c.Output) != "" {
			hasNonEmpty = true
			break
		}
	}
	if !hasNonEmpty {
		return ErrProposalInvalidField("至少需要一组有内容的样例")
	}
	return nil
}

// proposalErr 提议校验错误：统一映射到 400，但可携带中文消息。
type proposalErr struct{ msg string }

func (e *proposalErr) Error() string { return e.msg }

func ErrProposalInvalidField(msg string) error { return &proposalErr{msg: msg} }

func isProposalErr(err error) bool {
	_, ok := err.(*proposalErr)
	return ok
}

// proposalCreate 学生新建出题（pri）：进入 pending 状态等待老师/管理员审核。
func (s *Server) proposalCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req ProposalCreateReq
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := proposalValidate(&req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	authorName := ""
	_ = s.db.QueryRow(`SELECT COALESCE(NULLIF(realname,''), username) FROM users WHERE id=?`,
		claims.UserID).Scan(&authorName)
	res, err := s.db.Exec(`INSERT INTO problem_proposals(
		author_id, author_name, name, background, description,
		input_format, output_format, hint) VALUES(?,?,?,?,?,?,?,?)`,
		claims.UserID, authorName, req.Name, req.Background, req.Description,
		req.InputFormat, req.OutputFormat, req.Hint)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	for i, c := range req.Cases {
		if c.Input == "" && c.Output == "" {
			continue
		}
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO proposal_cases(proposal_id, index_no, input, output)
			VALUES(?,?,?,?)`, id, i+1, c.Input, c.Output)
	}
	logOp(s.db, claims, "proposal_create", strconv.FormatInt(id, 10), req.Name, clientIP(r))
	OK(w, map[string]any{"id": id, "status": PPStatusPending, "reward_default": ppRewardDefault})
}

// proposalListMine 当前用户的提议列表：置顶按 status 分桶展示（pending 优先）。
func (s *Server) proposalListMine(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, author_id, author_name, name, background, description,
		input_format, output_format, hint, status, review_comment, reviewer_id,
		reward_points, approved_problem_id, created_at FROM problem_proposals
		WHERE author_id=? ORDER BY
			CASE status WHEN 'pending' THEN 0 WHEN 'withdrawn' THEN 1
			WHEN 'rejected' THEN 2 ELSE 3 END,
			created_at DESC, id DESC LIMIT 200`, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ProposalResp{}
	for rows.Next() {
		var p ProposalResp
		var created time.Time
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Name, &p.Background,
			&p.Description, &p.InputFormat, &p.OutputFormat, &p.Hint, &p.Status,
			&p.ReviewComment, &p.ReviewerID, &p.RewardPoints, &p.ApprovedProblemID,
			(*time.Time)(&created)); err != nil {
			continue
		}
		p.CreatedAt = created.Format(time.RFC3339)
		list = append(list, p)
	}
	OK(w, map[string]any{"list": list, "total": len(list)})
}

// proposalDetailMine 提议详情（仅作者本人或审核员可看）。
func (s *Server) proposalDetailMine(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "提议编号无效")
		return
	}
	p, ok := s.proposalByID(id)
	if !ok {
		Fail(w, http.StatusNotFound, "提议不存在")
		return
	}
	isReviewer := RequireRole(claims, "teacher", "admin", "super", "superadmin")
	if p.AuthorID != claims.UserID && !isReviewer {
		Fail(w, http.StatusNotFound, "提议不存在")
		return
	}
	full, _ := s.proposalDetailFull(id)
	OK(w, full)
}

// proposalWithdraw 作者撤回提议，回到待修改状态。
func (s *Server) proposalWithdraw(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "提议编号无效")
		return
	}
	p, ok := s.proposalByID(id)
	if !ok || p.AuthorID != claims.UserID {
		Fail(w, http.StatusNotFound, "提议不存在")
		return
	}
	if p.Status != PPStatusPending && p.Status != PPStatusRejected {
		Fail(w, http.StatusBadRequest, "只有待审核或被拒绝的提议可撤回")
		return
	}
	_, err := s.db.Exec(`UPDATE problem_proposals SET status=? WHERE id=? AND author_id=?`,
		PPStatusWithdrawn, id, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "proposal_withdraw", strconv.FormatInt(id, 10), p.Name, clientIP(r))
	OK(w, map[string]any{"id": id, "status": PPStatusWithdrawn})
}

// proposalDelete 作者删除撤回/被拒绝的提议。
func (s *Server) proposalDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "提议编号无效")
		return
	}
	p, ok := s.proposalByID(id)
	if !ok || p.AuthorID != claims.UserID {
		Fail(w, http.StatusNotFound, "提议不存在")
		return
	}
	if p.Status == PPStatusPending || p.Status == PPStatusApproved {
		Fail(w, http.StatusBadRequest, "只有撤回或被拒绝的提议可删除")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM proposal_cases WHERE proposal_id=?`, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := s.db.Exec(`DELETE FROM problem_proposals WHERE id=? AND author_id=?`, id, claims.UserID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "proposal_delete", strconv.FormatInt(id, 10), p.Name, clientIP(r))
	OK(w, map[string]any{"id": id, "removed": true})
}

// proposalPendingList 老师/管理员看待审列表。
func (s *Server) proposalPendingList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEditorClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, author_id, author_name, name, background, description,
		input_format, output_format, hint, status, review_comment, reviewer_id,
		reward_points, approved_problem_id, created_at FROM problem_proposals
		ORDER BY created_at ASC, id ASC LIMIT 200`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ProposalResp{}
	for rows.Next() {
		var p ProposalResp
		var created time.Time
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Name, &p.Background,
			&p.Description, &p.InputFormat, &p.OutputFormat, &p.Hint, &p.Status,
			&p.ReviewComment, &p.ReviewerID, &p.RewardPoints, &p.ApprovedProblemID,
			(*time.Time)(&created)); err != nil {
			continue
		}
		p.CreatedAt = created.Format(time.RFC3339)
		list = append(list, p)
	}
	OK(w, map[string]any{"list": list, "total": len(list)})
}

// proposalReview 老师/管理员审核提议：
//
//	accepted=true → 转为 problem + cases，发放积分（默认 ppRewardDefault，可用 req.Points 覆盖）
//	accepted=false → 状态改为 rejected，可附审核意见
//
// 同一提议重复审核：approved/rejected 状态不可再改，避免重复发分。
func (s *Server) proposalReview(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireEditorClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "提议编号无效")
		return
	}
	full, ok := s.proposalDetailFull(id)
	if !ok {
		Fail(w, http.StatusNotFound, "提议不存在")
		return
	}
	if status, _ := full["status"].(string); status != "" && status != PPStatusPending {
		Fail(w, http.StatusBadRequest, "该提议已审核，不能重复操作")
		return
	}
	var body struct {
		Accepted *bool  `json:"accepted"`
		Points   int    `json:"reward_points"`
		Comment  string `json:"comment"`
	}
	if err := decode(r, &body); err != nil {
		Fail(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if body.Comment != "" {
		body.Comment = strings.TrimSpace(body.Comment)
	}
	// accepted 未传默认接受（保持接口简洁）
	accept := body.Accepted == nil || *body.Accepted
	if !accept {
		_, err := s.db.Exec(`UPDATE problem_proposals SET status=?, review_comment=?,
			reviewer_id=?, reviewed_at=? WHERE id=?`,
			PPStatusRejected, body.Comment, claims.UserID, time.Now().Format(time.RFC3339), id)
		if err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		name, _ := full["name"].(string)
		logOp(s.db, claims, "proposal_reject", strconv.FormatInt(id, 10), name, clientIP(r))
		OK(w, map[string]any{"id": id, "status": PPStatusRejected, "reward_points": 0})
		return
	}

	// 审核通过：转题库 + 发积分
	points := body.Points
	if points <= 0 {
		points = ppRewardDefault
	}
	problemName, _ := full["name"].(string)
	problemDesc, _ := full["description"].(string)
	problemHint, _ := full["hint"].(string)
	inputFormat, _ := full["input_format"].(string)
	outputFormat, _ := full["output_format"].(string)
	background, _ := full["background"].(string)

	// content 拼装：背景（如有）→ 描述 → 输入格式 → 输出格式
	var sb strings.Builder
	if background != "" {
		sb.WriteString("## 题目背景\n\n")
		sb.WriteString(background)
		sb.WriteString("\n\n")
	}
	sb.WriteString("## 题目描述\n\n")
	sb.WriteString(problemDesc)
	sb.WriteString("\n\n")
	if inputFormat != "" {
		sb.WriteString("## 输入格式\n\n")
		sb.WriteString(inputFormat)
		sb.WriteString("\n\n")
	}
	if outputFormat != "" {
		sb.WriteString("## 输出格式\n\n")
		sb.WriteString(outputFormat)
		sb.WriteString("\n\n")
	}
	content := sb.String()

	res, err := s.db.Exec(`INSERT INTO problems(name, difficulty, problem_type, time_limit, mem_limit,
		content, hint) VALUES(?,?,?,?,?,?,?)`,
		problemName, "Easy", "OJ", 1000, 256, content, problemHint)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	problemID, _ := res.LastInsertId()

	casesArr, _ := full["cases"].([]ProblemCase)
	for i, c := range casesArr {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO cases(problem_id, index_no, input, output)
			VALUES(?,?,?,?)`, problemID, i+1, c.Input, c.Output)
	}
	_, _ = s.db.Exec(`DELETE FROM proposal_cases WHERE proposal_id=?`, id)

	reviewedAt := time.Now().Format(time.RFC3339)
	_, err = s.db.Exec(`UPDATE problem_proposals SET status=?, review_comment=?,
		reviewer_id=?, reviewed_at=?, reward_points=?, approved_problem_id=? WHERE id=?`,
		PPStatusApproved, body.Comment, claims.UserID, reviewedAt, points, problemID, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	authorID, _ := full["author_id"].(int64)
	authorName, _ := full["author_name"].(string)
	remark := "学生出题审核通过：" + problemName
	_, _ = s.awardPoints(authorID, points, "admin", "problem", problemID, remark)

	logOp(s.db, claims, "proposal_approve", strconv.FormatInt(id, 10), problemName, clientIP(r))
	OK(w, map[string]any{
		"id": id, "status": PPStatusApproved,
		"problem_id": problemID, "reward_points": points,
		"author": map[string]any{"id": authorID, "name": authorName},
		"remark": remark,
	})
}

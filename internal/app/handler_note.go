package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 个人笔记：仅本人可见的私密备忘，用于记录题目思路、复盘、错题笔记等。
// 与「错题本」不同：错题本由提交自动归档；笔记由用户主动创建/编辑/删除，
// 内容完全本地写入，不做外部 URL 校验，也不对外公开。

// normalizeNote 规范化笔记输入：标题必填且 ≤ 80 字，正文 ≤ 10 万字，
// 标签按多种分隔符切分、去空、去重、最多 8 个、每个 ≤ 12 字。
func normalizeNote(title, content, tags string) (string, string, string, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	tags = strings.TrimSpace(tags)
	if title == "" || len([]rune(title)) > 80 {
		return "", "", "", sql.ErrNoRows
	}
	if len([]rune(content)) > 100_000 {
		return "", "", "", sql.ErrNoRows
	}
	seen := map[string]bool{}
	parts := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == '，' || r == ' ' || r == '、' || r == ';' || r == '；'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || len([]rune(p)) > 12 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= 8 {
			break
		}
	}
	return title, content, strings.Join(out, ","), nil
}

// splitTags 把逗号分隔的标签字符串切成切片；空串返回空数组而不是 nil。
func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// noteByID 读取单条笔记（仅本人），不存在或非本人返回 false。
func (s *Server) noteByID(id, userID int64) (*map[string]any, bool) {
	var title, content, tags string
	var pinned int
	var created, updated time.Time
	err := s.db.QueryRow(`SELECT title, content, tags, pinned, created_at, updated_at
		FROM notes WHERE id=? AND user_id=?`, id, userID).
		Scan(&title, &content, &tags, &pinned, (*time.Time)(&created), (*time.Time)(&updated))
	if err != nil {
		return nil, false
	}
	m := map[string]any{
		"id":         id,
		"title":      title,
		"content":    content,
		"tags":       splitTags(tags),
		"pinned":     pinned == 1,
		"created_at": created.Format(time.RFC3339),
		"updated_at": updated.Format(time.RFC3339),
	}
	return &m, true
}

// noteList 当前用户的笔记列表：置顶优先，其次按更新时间倒序，最多 200 条。
func (s *Server) noteList(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, title, content, tags, pinned, created_at, updated_at
		FROM notes WHERE user_id=? ORDER BY pinned DESC, updated_at DESC, id DESC LIMIT 200`,
		claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id int64
		var title, content, tags string
		var pinned int
		var created, updated time.Time
		if err := rows.Scan(&id, &title, &content, &tags, &pinned, (*time.Time)(&created), (*time.Time)(&updated)); err != nil {
			continue
		}
		snippet := content
		if runes := []rune(content); len(runes) > 120 {
			snippet = string(runes[:120]) + "…"
		}
		list = append(list, map[string]any{
			"id":         id,
			"title":      title,
			"snippet":    snippet,
			"tags":       splitTags(tags),
			"pinned":     pinned == 1,
			"created_at": created.Format(time.RFC3339),
			"updated_at": updated.Format(time.RFC3339),
		})
	}
	OK(w, map[string]any{"list": list, "total": len(list)})
}

// noteDetail 单条笔记详情（仅本人）。
func (s *Server) noteDetail(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "笔记编号无效")
		return
	}
	n, ok := s.noteByID(id, claims.UserID)
	if !ok {
		Fail(w, http.StatusNotFound, "笔记不存在")
		return
	}
	OK(w, *n)
}

// noteCreate 新建笔记。
func (s *Server) noteCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Tags    string `json:"tags"`
		Pinned  bool   `json:"pinned"`
	}
	if err := decode(r, &body); err != nil {
		Fail(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	title, content, tags, err := normalizeNote(body.Title, body.Content, body.Tags)
	if err != nil {
		Fail(w, http.StatusBadRequest, "标题不能为空或超长")
		return
	}
	res, err := s.db.Exec(`INSERT INTO notes(title, content, tags, pinned, user_id) VALUES(?,?,?,?,?)`,
		title, content, tags, boolInt(body.Pinned), claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "note_create", strconv.FormatInt(id, 10), title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// noteUpdate 全字段更新笔记（标题/正文/标签/置顶）。
func (s *Server) noteUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "笔记编号无效")
		return
	}
	if _, ok := s.noteByID(id, claims.UserID); !ok {
		Fail(w, http.StatusNotFound, "笔记不存在")
		return
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Tags    string `json:"tags"`
		Pinned  bool   `json:"pinned"`
	}
	if err := decode(r, &body); err != nil {
		Fail(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	title, content, tags, err := normalizeNote(body.Title, body.Content, body.Tags)
	if err != nil {
		Fail(w, http.StatusBadRequest, "标题不能为空或超长")
		return
	}
	res, err := s.db.Exec(`UPDATE notes SET title=?, content=?, tags=?, pinned=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?`,
		title, content, tags, boolInt(body.Pinned), id, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "笔记不存在")
		return
	}
	logOp(s.db, claims, "note_update", strconv.FormatInt(id, 10), title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// noteDelete 删除笔记（硬删除：笔记是纯用户数据，保留 0 天）。
func (s *Server) noteDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "笔记编号无效")
		return
	}
	n, ok := s.noteByID(id, claims.UserID)
	if !ok {
		Fail(w, http.StatusNotFound, "笔记不存在")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM notes WHERE id=? AND user_id=?`, id, claims.UserID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	title, _ := (*n)["title"].(string)
	logOp(s.db, claims, "note_delete", strconv.FormatInt(id, 10), title, clientIP(r))
	OK(w, map[string]any{"id": id, "removed": true})
}

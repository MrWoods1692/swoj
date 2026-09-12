package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 公告：管理员发布全站通知。列表默认只展示 visible=1，置顶优先、时间倒序；
// visible=0 即草稿，仅管理后台可见，公开端点与不存在同样 404，避免泄露存在性。
// level 白名单控制前端展示样式，非法值回落 info，不把任意字符串写进库。

const (
	noticeInfo    = "info"
	noticeSuccess = "success"
	noticeWarning = "warning"
	noticeDanger  = "danger"
)

// validNoticeLevel 公告级别白名单。
func validNoticeLevel(lv string) bool {
	switch lv {
	case noticeInfo, noticeSuccess, noticeWarning, noticeDanger:
		return true
	}
	return false
}

// NoticeReq 公告创建/更新请求。更新为全字段覆盖（公告体量小，无部分更新必要）。
type NoticeReq struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Level   string `json:"level"`
	Pinned  bool   `json:"pinned"`
	Visible bool   `json:"visible"`
}

// NoticeResp 公告列表/详情返回结构。
type NoticeResp struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	Level     string    `json:"level"`
	Pinned    bool      `json:"pinned"`
	Visible   bool      `json:"visible"`
	Views     int       `json:"views"`
	Creator   int64     `json:"creator"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// scanHead 扫描列表口径的列（不含 content）：
// id, title, level, pinned, visible, views, creator, created_at, updated_at。
func (n *NoticeResp) scanHead(rows interface{ Scan(...any) error }) error {
	return rows.Scan(&n.ID, &n.Title, &n.Level, &n.Pinned, &n.Visible,
		&n.Views, &n.Creator, (*time.Time)(&n.CreatedAt), (*time.Time)(&n.UpdatedAt))
}

// noticeDecode 校验公告请求体，返回规范化后的字段。
// 标题/正文为空或超长返回 ErrNoRows，由调用方统一转 400。
func noticeDecode(r *http.Request) (NoticeReq, error) {
	var req NoticeReq
	if err := decode(r, &req); err != nil {
		return req, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || len([]rune(req.Title)) > 120 || req.Content == "" {
		return req, sql.ErrNoRows
	}
	if !validNoticeLevel(req.Level) {
		req.Level = noticeInfo
	}
	return req, nil
}

// noticeList 公开公告列表：仅 visible=1，置顶在前、更新时间倒序。
func (s *Server) noticeList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(q.Get("size"), 10)
	if size < 1 || size > 100 {
		size = 10
	}
	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM notices WHERE visible=1`).Scan(&total)
	rows, err := s.db.Query(`SELECT id, title, level, pinned, visible, views, creator,
		created_at, updated_at FROM notices WHERE visible=1
		ORDER BY pinned DESC, updated_at DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []NoticeResp{}
	for rows.Next() {
		var n NoticeResp
		if err := n.scanHead(rows); err != nil {
			continue
		}
		list = append(list, n)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "size": size})
}

// noticeDetail 公告详情，公开可见项浏览量 +1；草稿与不存在同样 404。
func (s *Server) noticeDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "公告编号无效")
		return
	}
	var n NoticeResp
	err := s.db.QueryRow(`SELECT id, title, content, level, pinned, visible, views, creator,
		created_at, updated_at FROM notices WHERE id=? AND visible=1`, id).
		Scan(&n.ID, &n.Title, &n.Content, &n.Level, &n.Pinned, &n.Visible,
			&n.Views, &n.Creator, (*time.Time)(&n.CreatedAt), (*time.Time)(&n.UpdatedAt))
	if err == sql.ErrNoRows {
		Fail(w, http.StatusNotFound, "公告不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, _ = s.db.Exec(`UPDATE notices SET views = views + 1 WHERE id=?`, id)
	n.Views++
	OK(w, n)
}

// noticeAdminList 管理端列表：含草稿，置顶在前、更新时间倒序，分页。
func (s *Server) noticeAdminList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		size = 20
	}
	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM notices`).Scan(&total)
	rows, err := s.db.Query(`SELECT id, title, level, pinned, visible, views, creator,
		created_at, updated_at FROM notices
		ORDER BY pinned DESC, updated_at DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []NoticeResp{}
	for rows.Next() {
		var n NoticeResp
		if err := n.scanHead(rows); err != nil {
			continue
		}
		list = append(list, n)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "size": size})
}

// noticeCreate 管理员发布公告。visible=false 即存草稿。
func (s *Server) noticeCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	req, err := noticeDecode(r)
	if err == sql.ErrNoRows {
		Fail(w, http.StatusBadRequest, "标题（≤120字）与正文不能为空")
		return
	}
	if err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.db.Exec(`INSERT INTO notices(title, content, level, pinned, visible, creator)
		VALUES(?,?,?,?,?,?)`,
		req.Title, req.Content, req.Level, boolInt(req.Pinned), boolInt(req.Visible), claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "notice_create", strconv.FormatInt(id, 10), req.Title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// noticeUpdate 管理员全字段更新公告。
func (s *Server) noticeUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "公告编号无效")
		return
	}
	req, err := noticeDecode(r)
	if err == sql.ErrNoRows {
		Fail(w, http.StatusBadRequest, "标题（≤120字）与正文不能为空")
		return
	}
	if err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notices WHERE id=?`, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists == 0 {
		Fail(w, http.StatusNotFound, "公告不存在")
		return
	}
	_, err = s.db.Exec(`UPDATE notices SET title=?, content=?, level=?, pinned=?, visible=?,
		updated_at=datetime('now') WHERE id=?`,
		req.Title, req.Content, req.Level, boolInt(req.Pinned), boolInt(req.Visible), id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "notice_update", strconv.FormatInt(id, 10), req.Title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// noticeDelete 管理员删除公告（硬删，未部署无历史包袱）。
func (s *Server) noticeDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "公告编号无效")
		return
	}
	res, err := s.db.Exec(`DELETE FROM notices WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "公告不存在")
		return
	}
	logOp(s.db, claims, "notice_delete", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id})
}

package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 更新日志：管理员记录每次发布的改动，全站公开可见。
// 与公告（notice）的区别是这里按版本号组织、带改动分类，用于对外说明"这版改了什么"，
// 而不是站内通知。kind 白名单控制前端标签样式，非法值回落 feature，不把任意字符串写进库。

const (
	changeFeature     = "feature"
	changeFix         = "fix"
	changeImprove     = "improve"
	changePerf        = "perf"
	changeSecurity    = "security"
	changeRefactor    = "refactor"
	changeVersionKind = changeFeature
)

// changeLabels 分类到中文标签的映射，前端展示与筛选共用。
var changeLabels = map[string]string{
	changeFeature:  "新功能",
	changeFix:      "修复",
	changeImprove:  "改进",
	changePerf:     "性能",
	changeSecurity: "安全",
	changeRefactor: "重构",
}

// validChangeKind 改动分类白名单。
func validChangeKind(k string) bool {
	_, ok := changeLabels[k]
	return ok
}

// ChangeReq 更新日志创建/更新请求。
type ChangeReq struct {
	Version  string `json:"version"`
	Kind     string `json:"kind"`
	Content  string `json:"content"`
	Notes    string `json:"notes"`
	Released string `json:"released_at"` // YYYY-MM-DD，留空取当天
}

// ChangeResp 更新日志返回结构。
type ChangeResp struct {
	ID         int64  `json:"id"`
	Version    string `json:"version"`
	Kind       string `json:"kind"`
	KindLabel  string `json:"kind_label"`
	Content    string `json:"content"`
	Notes      string `json:"notes"`
	ReleasedAt string `json:"released_at"`
	Creator    int64  `json:"creator"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// changelogDecode 校验请求体并规范化字段。
// 版本号与正文为空返回 ErrNoRows，由调用方统一转 400。
func changelogDecode(r *http.Request) (ChangeReq, error) {
	var req ChangeReq
	if err := decode(r, &req); err != nil {
		return req, err
	}
	req.Version = strings.TrimSpace(req.Version)
	req.Content = strings.TrimSpace(req.Content)
	req.Notes = strings.TrimSpace(req.Notes)
	req.Kind = strings.TrimSpace(req.Kind)
	req.Released = strings.TrimSpace(req.Released)
	// 版本号形如 1.2.0 / v1.2 / 2026-09：只允许常见版本字符，避免入库脏值。
	if req.Version == "" || len([]rune(req.Version)) > 32 || req.Content == "" || len([]rune(req.Content)) > 120 {
		return req, sql.ErrNoRows
	}
	for _, c := range req.Version {
		if !((c >= '0' && c <= '9') || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '-' || c == '+' || c == '_') {
			return req, sql.ErrNoRows
		}
	}
	if !validChangeKind(req.Kind) {
		req.Kind = changeVersionKind
	}
	if req.Released == "" {
		req.Released = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", req.Released); err != nil {
		return req, sql.ErrNoRows
	}
	if len([]rune(req.Notes)) > 4000 {
		req.Notes = ""
	}
	return req, nil
}

func changeScan(rows interface{ Scan(...any) error }) (*ChangeResp, error) {
	n := &ChangeResp{}
	var created, updated time.Time
	if err := rows.Scan(&n.ID, &n.Version, &n.Kind, &n.Content, &n.Notes, &n.Creator,
		&n.ReleasedAt, (*time.Time)(&created), (*time.Time)(&updated)); err != nil {
		return nil, err
	}
	if lv, ok := changeLabels[n.Kind]; ok {
		n.KindLabel = lv
	} else {
		n.KindLabel = changeLabels[changeVersionKind]
		n.Kind = changeVersionKind
	}
	n.CreatedAt = created.Format("2006-01-02 15:04")
	n.UpdatedAt = updated.Format("2006-01-02 15:04")
	return n, nil
}

const changelogCols = `id, version, kind, content, notes, creator, released_at, created_at, updated_at`

// changelogList 公开更新日志列表：按发布时间倒序，同版本按 id 倒序。
func (s *Server) changelogList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		size = 20
	}
	kind := strings.TrimSpace(q.Get("kind"))
	if kind != "" && !validChangeKind(kind) {
		kind = ""
	}

	where := ""
	var args []any
	if kind != "" {
		where = " WHERE kind=?"
		args = append(args, kind)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM changelog`+where, args...).Scan(&total); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	sqlStr := `SELECT ` + changelogCols + ` FROM changelog` + where +
		` ORDER BY released_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	list := []ChangeResp{}
	kinds := map[string]int{}
	for rows.Next() {
		n, err := changeScan(rows)
		if err != nil {
			continue
		}
		list = append(list, *n)
		kinds[n.Kind]++
	}
	OK(w, map[string]any{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
		"kinds": kinds,
	})
}

// changelogDetail 公开单条详情，同时累加浏览计数。
func (s *Server) changelogDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "记录编号无效")
		return
	}
	rows, err := s.db.Query(`SELECT `+changelogCols+` FROM changelog WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	if !rows.Next() {
		Fail(w, http.StatusNotFound, "记录不存在")
		return
	}
	n, err := changeScan(rows)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, n)
}

// changelogAdminList 管理后台列表：含全部记录，按发布时间倒序。
func (s *Server) changelogAdminList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT ` + changelogCols + ` FROM changelog ORDER BY released_at DESC, id DESC LIMIT 200`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ChangeResp{}
	for rows.Next() {
		if n, err := changeScan(rows); err == nil {
			list = append(list, *n)
		}
	}
	OK(w, map[string]any{"list": list, "total": len(list)})
}

// changelogCreate 管理员新增一条更新日志。
func (s *Server) changelogCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	req, err := changelogDecode(r)
	if err == sql.ErrNoRows {
		Fail(w, http.StatusBadRequest, "版本号（≤32字）与标题（≤120字）不能为空")
		return
	}
	if err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.db.Exec(`INSERT INTO changelog(version, kind, content, notes, creator, released_at)
		VALUES(?,?,?,?,?,?)`, req.Version, req.Kind, req.Content, req.Notes, claims.UserID, req.Released)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "changelog_create", strconv.FormatInt(id, 10),
		req.Version+" "+req.Content, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// changelogUpdate 管理员全字段更新。
func (s *Server) changelogUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "记录编号无效")
		return
	}
	req, err := changelogDecode(r)
	if err == sql.ErrNoRows {
		Fail(w, http.StatusBadRequest, "版本号（≤32字）与标题（≤120字）不能为空")
		return
	}
	if err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM changelog WHERE id=?`, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists == 0 {
		Fail(w, http.StatusNotFound, "记录不存在")
		return
	}
	_, err = s.db.Exec(`UPDATE changelog SET version=?, kind=?, content=?, notes=?, released_at=?,
		updated_at=datetime('now') WHERE id=?`,
		req.Version, req.Kind, req.Content, req.Notes, req.Released, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "changelog_update", strconv.FormatInt(id, 10),
		req.Version+" "+req.Content, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// changelogDelete 管理员删除记录。
func (s *Server) changelogDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "记录编号无效")
		return
	}
	res, err := s.db.Exec(`DELETE FROM changelog WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "记录不存在")
		return
	}
	logOp(s.db, claims, "changelog_delete", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id})
}

// changelogMeta 返回分类白名单，前端表单与筛选据此渲染。
func (s *Server) changelogMeta(w http.ResponseWriter, r *http.Request) {
	kinds := []map[string]string{}
	for _, k := range []string{changeFeature, changeFix, changeImprove, changePerf, changeSecurity, changeRefactor} {
		kinds = append(kinds, map[string]string{"kind": k, "label": changeLabels[k]})
	}
	OK(w, map[string]any{"kinds": kinds})
}

package app

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 学习资料：由老师与管理员发布，全站可见。资料只是一条外部链接的引用，
// 不做附件存储——url 只允许 http/https，避免把站内相对路径或协议滥用地址写进资料库。
// icon 与 category 都走白名单，非法值回落默认值而不是拒绝请求，录入体验更顺。

// materialIcons 图标白名单，与前端 MATERIAL_ICONS 一一对应。
var materialIcons = map[string]bool{
	"book": true, "doc": true, "video": true, "code": true,
	"link": true, "note": true, "image": true, "question": true,
}

// materialCategories 资料分类，首项空串表示「全部/未分类」。
var materialCategories = []string{"", "入门", "进阶", "算法", "题解", "课件", "工具", "比赛"}

func materialCatSet() map[string]bool {
	m := map[string]bool{}
	for _, c := range materialCategories {
		m[c] = true
	}
	return m
}

// validMaterialURL 校验资料地址：仅 http/https、主机非空且含点、不含空白与引号尖括号。
func validMaterialURL(u string) bool {
	if u == "" || len([]rune(u)) > 500 || strings.ContainsAny(u, " \t\n\r\"'<>") {
		return false
	}
	rest := ""
	low := strings.ToLower(u)
	if strings.HasPrefix(low, "http://") {
		rest = u[len("http://"):]
	} else if strings.HasPrefix(low, "https://") {
		rest = u[len("https://"):]
	} else {
		return false
	}
	host := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host = rest[:i]
	}
	return host != "" && strings.Contains(host, ".")
}

// MaterialReq 资料创建/更新请求。
type MaterialReq struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
	Desc     string `json:"desc"`
	Pinned   bool   `json:"pinned"`
}

// MaterialResp 资料返回结构，Creator 为发布人昵称（realname 优先，回落 username）。
type MaterialResp struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	URL       string    `json:"url"`
	Icon      string    `json:"icon"`
	Desc      string    `json:"desc"`
	Pinned    bool      `json:"pinned"`
	Clicks    int       `json:"clicks"`
	CreatorID int64     `json:"creator_id"`
	Creator   string    `json:"creator"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// materialSelectCols 列表与详情共用的 SELECT 列，JOIN 发布人昵称。
const materialSelectCols = `m.id, m.title, m.category, m.url, m.icon, m.desc, m.pinned, m.clicks,
	m.creator_id, COALESCE(u.realname,''), COALESCE(u.username,''), m.created_at, m.updated_at`

const materialFrom = `FROM materials m LEFT JOIN users u ON u.id=m.creator_id`

// materialDecode 校验请求体并规范化字段：标题与链接必填，分类图标回落默认值。
func materialDecode(r *http.Request) (MaterialReq, error) {
	var req MaterialReq
	if err := decode(r, &req); err != nil {
		return req, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.URL = strings.TrimSpace(req.URL)
	req.Desc = strings.TrimSpace(req.Desc)
	req.Category = strings.TrimSpace(req.Category)
	req.Icon = strings.TrimSpace(req.Icon)
	if req.Title == "" || len([]rune(req.Title)) > 80 || !validMaterialURL(req.URL) {
		return req, sql.ErrNoRows
	}
	if !materialCatSet()[req.Category] {
		req.Category = ""
	}
	if !materialIcons[req.Icon] {
		req.Icon = "book"
	}
	return req, nil
}

// materialByID 按 id 读取单条资料，不存在返回 false。
func (s *Server) materialByID(id int64) (*MaterialResp, bool) {
	rows, err := s.db.Query(`SELECT `+materialSelectCols+` `+materialFrom+` WHERE m.id=?`, id)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, false
	}
	var m MaterialResp
	var realname, username string
	if err := rows.Scan(&m.ID, &m.Title, &m.Category, &m.URL, &m.Icon, &m.Desc, &m.Pinned,
		&m.Clicks, &m.CreatorID, &realname, &username,
		(*time.Time)(&m.CreatedAt), (*time.Time)(&m.UpdatedAt)); err != nil {
		return nil, false
	}
	m.Creator = realname
	if m.Creator == "" {
		m.Creator = username
	}
	return &m, true
}

// materialRows 把查询结果读成切片。
func (s *Server) materialRows(q string, args ...any) ([]*MaterialResp, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []*MaterialResp{}
	for rows.Next() {
		var m MaterialResp
		var realname, username string
		if err := rows.Scan(&m.ID, &m.Title, &m.Category, &m.URL, &m.Icon, &m.Desc, &m.Pinned,
			&m.Clicks, &m.CreatorID, &realname, &username,
			(*time.Time)(&m.CreatedAt), (*time.Time)(&m.UpdatedAt)); err != nil {
			continue
		}
		m.Creator = realname
		if m.Creator == "" {
			m.Creator = username
		}
		list = append(list, &m)
	}
	return list, nil
}

// materialsList 公开资料列表：分类可筛选，置顶优先、点击量高者优先、时间倒序。
func (s *Server) materialsList(w http.ResponseWriter, r *http.Request) {
	cat := strings.TrimSpace(r.URL.Query().Get("category"))
	if cat != "" && !materialCatSet()[cat] {
		cat = ""
	}
	q := `SELECT ` + materialSelectCols + ` ` + materialFrom
	args := []any{}
	if cat != "" {
		q += ` WHERE m.category=?`
		args = append(args, cat)
	}
	q += ` ORDER BY m.pinned DESC, m.clicks DESC, m.created_at DESC LIMIT 200`
	list, err := s.materialRows(q, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	cats := map[string]int{}
	if rows, err := s.db.Query(`SELECT category, COUNT(*) FROM materials GROUP BY category`); err == nil {
		for rows.Next() {
			var c string
			var n int
			if err := rows.Scan(&c, &n); err == nil {
				cats[c] = n
			}
		}
		rows.Close()
	}
	OK(w, map[string]any{"list": list, "categories": cats, "total": len(list)})
}

// materialDetail 单条详情并把点击量 +1（前端跳转前调用，用于统计访问量）。
func (s *Server) materialDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "资料编号无效")
		return
	}
	m, ok := s.materialByID(id)
	if !ok {
		Fail(w, http.StatusNotFound, "资料不存在")
		return
	}
	_, _ = s.db.Exec(`UPDATE materials SET clicks=clicks+1 WHERE id=?`, id)
	m.Clicks++
	OK(w, m)
}

// materialCreate 新增资料，需老师或管理员。
func (s *Server) materialCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireEditorClaims(w, r)
	if !ok {
		return
	}
	req, err := materialDecode(r)
	if err != nil {
		Fail(w, http.StatusBadRequest, "资料标题或链接无效")
		return
	}
	res, err := s.db.Exec(`INSERT INTO materials(title,category,url,icon,desc,pinned,creator_id)
		VALUES(?,?,?,?,?,?,?)`, req.Title, req.Category, req.URL, req.Icon, req.Desc,
		boolInt(req.Pinned), claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "material_create", strconv.FormatInt(id, 10), req.Title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// materialUpdate 全字段更新资料。
func (s *Server) materialUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireEditorClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "资料编号无效")
		return
	}
	req, err := materialDecode(r)
	if err != nil {
		Fail(w, http.StatusBadRequest, "资料标题或链接无效")
		return
	}
	res, err := s.db.Exec(`UPDATE materials SET title=?,category=?,url=?,icon=?,desc=?,pinned=?
		WHERE id=?`, req.Title, req.Category, req.URL, req.Icon, req.Desc,
		boolInt(req.Pinned), id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "资料不存在")
		return
	}
	logOp(s.db, claims, "material_update", strconv.FormatInt(id, 10), req.Title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// materialDelete 删除资料。
func (s *Server) materialDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireEditorClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "资料编号无效")
		return
	}
	m, ok := s.materialByID(id)
	if !ok {
		Fail(w, http.StatusNotFound, "资料不存在")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM materials WHERE id=?`, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "material_delete", strconv.FormatInt(id, 10), m.Title, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// materialsAdminList 管理端列表：含未置顶与低点击资料，按时间倒序，附白名单。
func (s *Server) materialsAdminList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEditorClaims(w, r); !ok {
		return
	}
	list, err := s.materialRows(`SELECT ` + materialSelectCols + ` ` + materialFrom +
		` ORDER BY m.created_at DESC, m.id DESC LIMIT 500`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"list": list, "total": len(list),
		"categories": materialCategories, "icons": materialIconNames()})
}

// materialsMeta 公开返回分类与图标白名单，前端据此渲染表单与标签栏。
func (s *Server) materialsMeta(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{"categories": materialCategories, "icons": materialIconNames()})
}

// materialIconNames 图标白名单切片，排序后返回保证顺序稳定。
func materialIconNames() []string {
	names := make([]string, 0, len(materialIcons))
	for k := range materialIcons {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

package app

import (
	"net/http"
	"strconv"
	"time"
)

// DiscussionResp 讨论列表与详情响应。
type DiscussionResp struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	ProblemID   int64  `json:"problem_id"`
	ProblemName string `json:"problem_name"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	LikeCount   int    `json:"like_count"`
	Replies     int    `json:"replies"`
	Liked       bool   `json:"liked"`
	CreatedAt   string `json:"created_at"`
}

// ReplyItem 讨论回复。
type ReplyItem struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// discussionList 分页返回讨论列表，支持按题目与关键词筛选。
func (s *Server) discussionList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	size := atoiDefault(q.Get("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where, args := []string{}, []any{}
	if id, err := strconv.ParseInt(q.Get("problem_id"), 10, 64); err == nil && id > 0 {
		where = append(where, "d.problem_id=?")
		args = append(args, id)
	}
	if kw := q.Get("keyword"); kw != "" {
		where = append(where, "(d.title LIKE ? OR d.content LIKE ?)")
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + stringsJoin(where, " AND ")
	}

	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM discussions d `+cond, args...).Scan(&total)

	rows, err := s.db.Query(`SELECT d.id, d.user_id, d.username, d.problem_id, d.title, d.content,
		d.like_count, d.created_at FROM discussions d `+cond+
		` ORDER BY d.created_at DESC, d.id DESC LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	uid := viewerID(r)
	likedSet := map[int64]bool{}
	if uid > 0 {
		rs, err := s.db.Query(`SELECT discussion_id FROM discussion_likes WHERE user_id=?`, uid)
		if err == nil {
			for rs.Next() {
				var id int64
				if rs.Scan(&id) == nil {
					likedSet[id] = true
				}
			}
			rs.Close()
		}
	}

	var list []DiscussionResp
	var replyIDs []int64
	var problemIDs []int64
	for rows.Next() {
		var d DiscussionResp
		var created time.Time
		if err := rows.Scan(&d.ID, &d.UserID, &d.Username, &d.ProblemID, &d.Title, &d.Content,
			&d.LikeCount, (*time.Time)(&created)); err != nil {
			continue
		}
		d.CreatedAt = created.String()
		list = append(list, d)
		replyIDs = append(replyIDs, d.ID)
		if d.ProblemID > 0 {
			problemIDs = append(problemIDs, d.ProblemID)
		}
	}
	rows.Close()

	// SQLite 单连接：回复数与题目名在扫完后统一查询。
	for i := range list {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM discussion_replies WHERE discussion_id=?`, list[i].ID).Scan(&list[i].Replies)
		if list[i].ProblemID > 0 {
			_ = s.db.QueryRow(`SELECT name FROM problems WHERE id=?`, list[i].ProblemID).Scan(&list[i].ProblemName)
		}
		list[i].Liked = likedSet[list[i].ID]
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "page_size": size})
}

// discussionDetail 返回单条讨论及其全部回复。
func (s *Server) discussionDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "讨论编号无效")
		return
	}
	var d DiscussionResp
	var created time.Time
	err := s.db.QueryRow(`SELECT id, user_id, username, problem_id, title, content, like_count,
		created_at FROM discussions WHERE id=?`, id).
		Scan(&d.ID, &d.UserID, &d.Username, &d.ProblemID, &d.Title, &d.Content, &d.LikeCount,
			(*time.Time)(&created))
	if err != nil {
		Fail(w, http.StatusNotFound, "讨论不存在")
		return
	}
	d.CreatedAt = created.String()
	if d.ProblemID > 0 {
		_ = s.db.QueryRow(`SELECT name FROM problems WHERE id=?`, d.ProblemID).Scan(&d.ProblemName)
	}

	rr, err := s.db.Query(`SELECT id, user_id, username, content, created_at FROM discussion_replies
		WHERE discussion_id=? ORDER BY created_at, id`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rr.Close()
	replies := []ReplyItem{}
	for rr.Next() {
		var it ReplyItem
		var c time.Time
		if err := rr.Scan(&it.ID, &it.UserID, &it.Username, &it.Content, (*time.Time)(&c)); err != nil {
			continue
		}
		it.CreatedAt = c.String()
		replies = append(replies, it)
	}
	OK(w, map[string]any{"discussion": d, "replies": replies})
}

// discussionCreate 发布新讨论帖。
func (s *Server) discussionCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		ProblemID int64  `json:"problem_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Title == "" || req.Content == "" {
		Fail(w, http.StatusBadRequest, "标题与内容不能为空")
		return
	}
	var u User
	row := s.db.QueryRow(`SELECT id, username FROM users WHERE id=?`, claims.UserID)
	if err := row.Scan(&u.ID, &u.Username); err != nil {
		Fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	var pName string
	if req.ProblemID > 0 {
		if err := s.db.QueryRow(`SELECT name FROM problems WHERE id=?`, req.ProblemID).Scan(&pName); err != nil {
			Fail(w, http.StatusNotFound, "题目不存在")
			return
		}
	}
	res, err := s.db.Exec(`INSERT INTO discussions(user_id, username, problem_id, title, content)
		VALUES(?,?,?,?,?)`, u.ID, u.Username, req.ProblemID, req.Title, req.Content)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "discussion_create", strconv.FormatInt(id, 10), req.Title, clientIP(r))
	s.refreshAchievements(claims.UserID)
	OK(w, map[string]any{"id": id})
}

// discussionReply 回复某条讨论。
func (s *Server) discussionReply(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	did, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "讨论编号无效")
		return
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM discussions WHERE id=?`, did).Scan(&n)
	if n == 0 {
		Fail(w, http.StatusNotFound, "讨论不存在")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Content == "" {
		Fail(w, http.StatusBadRequest, "回复内容不能为空")
		return
	}
	var u User
	if err := s.db.QueryRow(`SELECT id, username FROM users WHERE id=?`, claims.UserID).
		Scan(&u.ID, &u.Username); err != nil {
		Fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	res, err := s.db.Exec(`INSERT INTO discussion_replies(discussion_id, user_id, username, content)
		VALUES(?,?,?,?)`, did, u.ID, u.Username, req.Content)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	OK(w, map[string]any{"id": id})
}

// discussionLike 点赞或取消点赞，用局部事务保证计数与去重表一致。
func (s *Server) discussionLike(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	did, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "讨论编号无效")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM discussion_likes WHERE user_id=? AND discussion_id=?`,
		claims.UserID, did).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists > 0 {
		if _, err := tx.Exec(`DELETE FROM discussion_likes WHERE user_id=? AND discussion_id=?`,
			claims.UserID, did); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tx.Exec(`UPDATE discussions SET like_count = like_count - 1 WHERE id=?`, did); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := tx.Commit(); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]any{"liked": false})
		return
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO discussion_likes(user_id, discussion_id) VALUES(?,?)`,
		claims.UserID, did); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(`UPDATE discussions SET like_count = like_count + 1 WHERE id=?`, did); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"liked": true})
}

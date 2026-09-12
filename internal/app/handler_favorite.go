package app

import (
	"net/http"
	"strconv"
)

// 收藏：题目与训练计划共用一张 favorites(user_id, type, target_id)。
// type 白名单限定为 problem / plan，避免把收藏当成任意表的写入通道。
// 目标行被硬删后，列表经 JOIN 自然消失；不可见目标同样被 JOIN 过滤，
// 收藏不是看到隐藏内容的后门。

const (
	favProblem = "problem"
	favPlan    = "plan"
)

// favTarget 收藏类型对应的目标表与可见性条件（列名两表不同：
// problems 用 invisible=0，training_plans 用 visible=1）。
func favTarget(typ string) (table, visible string) {
	switch typ {
	case favProblem:
		return "problems", "f.invisible=0"
	case favPlan:
		return "training_plans", "f.visible=1"
	}
	return "", ""
}

// favoriteToggle 收藏或取消收藏（开关式，与讨论点赞同构）。
// 收藏计数列的增减与去重表写入放同一事务，口径一致。
func (s *Server) favoriteToggle(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	typ := r.PathValue("type")
	table, visible := favTarget(typ)
	if table == "" {
		Fail(w, http.StatusBadRequest, "收藏类型仅支持 problem 或 plan")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(w, http.StatusBadRequest, "目标编号无效")
		return
	}
	// 目标必须存在且对普通用户可见：隐藏题/不可见计划本就不展示，收藏同样拒绝，
	// 避免把计数写在看不到处的行上。
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` f WHERE f.id=? AND `+visible, id).Scan(&n); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == 0 {
		Fail(w, http.StatusNotFound, "收藏目标不存在")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM favorites WHERE user_id=? AND type=? AND target_id=?`,
		claims.UserID, typ, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists > 0 {
		if _, err := tx.Exec(`DELETE FROM favorites WHERE user_id=? AND type=? AND target_id=?`,
			claims.UserID, typ, id); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tx.Exec(`UPDATE `+table+` SET favorite_count = favorite_count - 1 WHERE id=? AND favorite_count > 0`, id); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := tx.Commit(); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]any{"favorited": false})
		return
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO favorites(user_id, type, target_id) VALUES(?,?,?)`,
		claims.UserID, typ, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(`UPDATE `+table+` SET favorite_count = favorite_count + 1 WHERE id=?`, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"favorited": true})
}

// favItem 收藏列表的一行：type 区分题目与计划，按类型填充对应字段集。
type favItem struct {
	Type        string `json:"type"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Info        string `json:"info,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
	Accept      int    `json:"accept,omitempty"`
	Submit      int    `json:"submit,omitempty"`
	FavoritedAt string `json:"favorited_at"`
}

// favoriteList 当前用户在某类型下的收藏列表，JOIN 目标表并过滤不可见项。
func (s *Server) favoriteList(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	typ := r.PathValue("type")
	table, visible := favTarget(typ)
	if table == "" {
		Fail(w, http.StatusBadRequest, "收藏类型仅支持 problem 或 plan")
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

	cols := "f.id, f.name, '', f.difficulty, f.accept, f.submit"
	if typ == favPlan {
		cols = "f.id, f.name, f.info, '', 0, 0"
	}
	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM favorites v JOIN `+table+` f ON f.id=v.target_id
		WHERE v.user_id=? AND v.type=? AND `+visible, claims.UserID, typ).Scan(&total)

	rows, err := s.db.Query(`SELECT `+cols+`, datetime(v.created_at)
		FROM favorites v JOIN `+table+` f ON f.id=v.target_id
		WHERE v.user_id=? AND v.type=? AND `+visible+`
		ORDER BY v.created_at DESC LIMIT ? OFFSET ?`,
		claims.UserID, typ, size, (page-1)*size)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []favItem{}
	for rows.Next() {
		var it favItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Info, &it.Difficulty, &it.Accept, &it.Submit, &it.FavoritedAt); err != nil {
			continue
		}
		it.Type = typ
		list = append(list, it)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "size": size})
}

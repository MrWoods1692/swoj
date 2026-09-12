package app

import (
	"net/http"
	"strconv"
	"strings"
)

// adminPointsAdjust 教师/管理员直接给用户增减积分。
func (s *Server) adminPointsAdjust(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Delta    int    `json:"delta"` // 正数加分，负数扣分
		Remark   string `json:"remark"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		Fail(w, http.StatusBadRequest, "用户名不能为空")
		return
	}
	if req.Delta == 0 {
		Fail(w, http.StatusBadRequest, "调整值不能为 0")
		return
	}

	var uid int64
	if err := s.db.QueryRow(`SELECT id FROM users WHERE username=?`, req.Username).Scan(&uid); err != nil {
		Fail(w, http.StatusNotFound, "用户不存在")
		return
	}

	// 不允许扣成负分：用条件更新一次性完成扣减，失败再回退库存。
	up, err := s.db.Exec(`UPDATE users SET points=points+? WHERE id=? AND points+?>=0`,
		req.Delta, uid, req.Delta)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := up.RowsAffected(); n == 0 {
		balance, _ := s.userPoints(uid)
		Fail(w, http.StatusConflict, "调整后积分为负数，当前余额 "+strconv.Itoa(balance))
		return
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "管理员调整"
	}
	if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, uid, req.Delta, CategoryAdmin, "admin", uid, remark); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	balance, err := s.userPoints(uid)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "points_adjust", req.Username,
		"delta="+strconv.Itoa(req.Delta), clientIP(r))
	OK(w, map[string]any{"username": req.Username, "delta": req.Delta, "balance": balance})
}

// adminPointsGrant 批量给用户加积分，返回成功条数。
func (s *Server) adminPointsGrant(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Usernames []string `json:"usernames"`
		Delta     int      `json:"delta"`
		Remark    string   `json:"remark"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Usernames) == 0 {
		Fail(w, http.StatusBadRequest, "用户列表不能为空")
		return
	}
	if len(req.Usernames) > 500 {
		Fail(w, http.StatusBadRequest, "单次最多处理 500 人")
		return
	}
	if req.Delta == 0 {
		Fail(w, http.StatusBadRequest, "调整值不能为 0")
		return
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "批量调整"
	}
	okCount := 0
	failed := []string{}
	for _, name := range req.Usernames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var uid int64
		if err := s.db.QueryRow(`SELECT id FROM users WHERE username=?`, name).Scan(&uid); err != nil {
			failed = append(failed, name)
			continue
		}
		bal, err := s.awardPoints(uid, req.Delta, CategoryAdmin, "admin", uid, remark)
		if err != nil || bal == 0 {
			failed = append(failed, name)
			continue
		}
		okCount++
	}
	logOp(s.db, claims, "points_grant_batch", "",
		"ok="+strconv.Itoa(okCount)+" delta="+strconv.Itoa(req.Delta), clientIP(r))
	OK(w, map[string]any{"ok": okCount, "failed": failed, "delta": req.Delta})
}

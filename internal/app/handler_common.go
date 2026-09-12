package app

import (
	"net/http"
	"strconv"
)

// ProblemBrief 题目精简信息，供比赛、作业、训练计划等列表复用。
type ProblemBrief struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
	OrderNo    int    `json:"order_no"`
	Accept     int    `json:"accept"`
	Submit     int    `json:"submit"`
}

// briefProblems 扫描一组题目为精简列表，列顺序固定为
// id, name, difficulty, order_no, accept, submit。
func briefProblems(db *DB, query string, args ...any) ([]ProblemBrief, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ProblemBrief{}
	for rows.Next() {
		var p ProblemBrief
		if err := rows.Scan(&p.ID, &p.Name, &p.Difficulty, &p.OrderNo, &p.Accept, &p.Submit); err != nil {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

// pathID 从路径参数解析正整数主键。
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// viewerID 取当前登录用户 ID，未登录返回 0。
func viewerID(r *http.Request) int64 {
	if claims, ok := ClaimsFrom(r); ok {
		return claims.UserID
	}
	return 0
}

// requireClaims 取当前用户声明；未登录时写出 401 并返回 false。
func requireClaims(w http.ResponseWriter, r *http.Request) (*Claims, bool) {
	claims, ok := ClaimsFrom(r)
	if !ok {
		Fail(w, http.StatusUnauthorized, "请先登录")
		return nil, false
	}
	return claims, true
}

// requireAdmin 校验管理员角色；失败时写出 403 并返回 false。
func requireAdmin(w http.ResponseWriter, claims *Claims) bool {
	if !RequireRole(claims, "admin", "super", "superadmin") {
		Fail(w, http.StatusForbidden, "需要管理员权限")
		return false
	}
	return true
}

// userByID 按 ID 读取用户，不包含密码字段。
func (s *Server) userByID(id int64) (User, bool) {
	var u User
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature,
		problem_count, rank_no, can_submit FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.ProblemCount, &u.RankNo, &u.CanSubmit)
	return u, err == nil
}

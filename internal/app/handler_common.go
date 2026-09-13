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
// 仅用于无需写操作日志的读接口；写接口请用 requireAdminClaims 以获取 claims。
func requireAdmin(w http.ResponseWriter, claims *Claims) bool {
	if !RequireRole(claims, "admin", "super", "superadmin") {
		Fail(w, http.StatusForbidden, "需要管理员权限")
		return false
	}
	return true
}

// requireAdminClaims 登录 + 管理员校验一步完成，供需要审计日志的写接口使用。
func requireAdminClaims(w http.ResponseWriter, r *http.Request) (*Claims, bool) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return nil, false
	}
	if !RequireRole(claims, "admin", "super", "superadmin") {
		Fail(w, http.StatusForbidden, "需要管理员权限")
		return nil, false
	}
	return claims, true
}

// requireEditorClaims 登录 + 编辑角色校验：老师与管理员都可维护资料类内容。
// 与 requireAdminClaims 的区别是放行 teacher，用于资料、课件等由任课老师发布的场景。
func requireEditorClaims(w http.ResponseWriter, r *http.Request) (*Claims, bool) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return nil, false
	}
	if !RequireRole(claims, "teacher", "admin", "super", "superadmin") {
		Fail(w, http.StatusForbidden, "需要老师或管理员权限")
		return nil, false
	}
	return claims, true
}

// userByID 按 ID 读取用户，不包含密码字段。
// 头像列不再由用户填写，这里统一补齐 QQ 派生值：
// 否则 /api/auth/me 返回空头像，顶栏只能退化为字母占位。
func (s *Server) userByID(id int64) (User, bool) {
	var u User
	err := s.db.QueryRow(`SELECT id, username, email, realname, role, school, avatar, signature, website, background, qq,
		problem_count, rank_no, can_submit, points, level FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Username, &u.Email, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.Website, &u.Background, &u.QQ,
			&u.ProblemCount, &u.RankNo, &u.CanSubmit, &u.Points, &u.Level)
	if u.Avatar == "" {
		u.Avatar = qqAvatarURL(u.QQ)
	}
	return u, err == nil
}

package app

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 个人主页（公开）：任何访客可查看任何用户的资料与统计，
// 只返回可公开展示的字段，不含 email、OAuth 绑定标识等隐私。
// 外链访问链接 = HostURL + /profile/{id}，前端路由与之一致。

// qqNumber QQ 号为 5-11 位数字（首位非 0），用于展示与拼接头像外链。
var qqNumber = regexp.MustCompile(`^[1-9][0-9]{4,10}$`)

// qqAvatarURL 腾讯官方头像直链；QQ 为空返回空串，由前端回落默认头像。
func qqAvatarURL(qq string) string {
	if !qqNumber.MatchString(qq) {
		return ""
	}
	return "https://q.qlogo.cn/g?b=qq&nk=" + qq + "&s=640"
}

// HeatCell 做题热力图的一天。
type HeatCell struct {
	Date     string `json:"date"`
	Total    int    `json:"total"`
	Accepted int    `json:"accepted"`
}

// userHomepage 公开个人主页聚合：资料 + AC/总提交/通过率 + 积分/等级/成就/排名
// + 注册与最近上线时间 + 活跃度 + 365 天做题热力图 + 算力使用量 + 外链。
func (s *Server) userHomepage(w http.ResponseWriter, r *http.Request) {
	uid, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	var u User
	err := s.db.QueryRow(`SELECT id, username, realname, role, school, avatar, signature,
		website, background, qq, points, level, created_at, last_login_at
		FROM users WHERE id=?`, uid).
		Scan(&u.ID, &u.Username, &u.RealName, &u.Role, &u.School, &u.Avatar, &u.Signature,
			&u.Website, &u.Background, &u.QQ, &u.Points, &u.Level,
			(*time.Time)(&u.CreatedAt), (*time.Time)(&u.LastLoginAt))
	if err == sql.ErrNoRows {
		Fail(w, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// AC 与总提交：通过率按提交条数计（区别于排行榜/完成率的按去重题数）。
	var accepted, solved, submitted int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=?`, uid).Scan(&submitted)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=? AND status=?`,
		uid, StatusAccepted).Scan(&accepted)
	_ = s.db.QueryRow(`SELECT COUNT(DISTINCT problem_id) FROM submissions WHERE user_id=? AND status=?`,
		uid, StatusAccepted).Scan(&solved)

	// 排名与 myStats 同口径：AC 去重题数严格更多的人数 + 1。
	var rank int
	_ = s.db.QueryRow(`SELECT COUNT(*)+1 FROM (SELECT user_id, COUNT(DISTINCT problem_id) AS a
		FROM submissions WHERE status=? GROUP BY user_id) x WHERE x.a > ?`,
		StatusAccepted, solved).Scan(&rank)

	// 算力使用量：测评实际耗时与内存峰值（毫秒 / KB）。
	var judgeTotal, judgeMax, memMax int
	var judgeAvg sql.NullFloat64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(time_used),0), COALESCE(MAX(time_used),0),
		COALESCE(MAX(mem_used),0), AVG(time_used)
		FROM submissions WHERE user_id=? AND status>?`, uid, StatusPending).
		Scan(&judgeTotal, &judgeMax, &memMax, &judgeAvg)

	// 活跃度：近 30 天有提交或有在线记录的自然日数。
	var activeDays int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM (
		SELECT DISTINCT date(created_at) AS d FROM submissions
			WHERE user_id=? AND created_at >= datetime('now','-29 days')
		UNION
		SELECT DISTINCT date FROM user_online_days
			WHERE user_id=? AND date >= date('now','-29 days'))`, uid, uid).Scan(&activeDays)

	// 做题热力图：近 365 天逐日提交量与 AC 量，无提交的日期不返回，前端按日期落格。
	heat := []HeatCell{}
	rows, err := s.db.Query(`SELECT date(created_at) AS d, COUNT(*),
		SUM(CASE WHEN status=? THEN 1 ELSE 0 END)
		FROM submissions WHERE user_id=? AND created_at >= datetime('now','-364 days')
		GROUP BY d ORDER BY d`, StatusAccepted, uid)
	if err == nil {
		for rows.Next() {
			var hc HeatCell
			if err := rows.Scan(&hc.Date, &hc.Total, &hc.Accepted); err != nil {
				continue
			}
			heat = append(heat, hc)
		}
		rows.Close()
	}

	// 成就：定义静态在代码里，这里返回该用户已解锁的清单。
	achList := []map[string]any{}
	var achCount int
	if arows, err := s.db.Query(`SELECT code, unlocked_at FROM user_achievements
		WHERE user_id=? ORDER BY unlocked_at`, uid); err == nil {
		for arows.Next() {
			var code string
			var at time.Time
			if err := arows.Scan(&code, (*time.Time)(&at)); err != nil {
				continue
			}
			achCount++
			name, icon := "", ""
			for _, d := range AchievementList {
				if d.Code == code {
					name, icon = d.Name, d.Icon
					break
				}
			}
			achList = append(achList, map[string]any{
				"code": code, "name": name, "icon": icon, "unlocked_at": at,
			})
		}
		arows.Close()
	}

	lv := levelInfoOf(s.cfg.Levels, u.Level)
	avatar := strings.TrimSpace(u.Avatar)
	if avatar == "" {
		avatar = qqAvatarURL(u.QQ)
	}
	OK(w, map[string]any{
		"user": map[string]any{
			"id": u.ID, "username": u.Username, "realname": u.RealName,
			"signature": u.Signature, "school": u.School,
			"avatar": avatar, "qq_avatar": qqAvatarURL(u.QQ),
			"website": u.Website, "background": u.Background, "qq": u.QQ,
		},
		"stats": map[string]any{
			"accepted": accepted, "submitted": submitted, "solved": solved,
			"pass_rate": fmtPct(accepted, submitted),
			"points":    u.Points, "level": lv, "rank": rank,
			"achievements":      achList,
			"achievement_core":  achCount,
			"achievement_total": len(AchievementList),
			"active_days_30":    activeDays,
			"registered_at":     u.CreatedAt,
			"last_login_at":     u.LastLoginAt,
		},
		"heatmap": heat,
		"compute": map[string]any{
			"total_ms": judgeTotal, "max_ms": judgeMax,
			"avg_ms": judgeAvg.Float64, "max_mem_kb": memMax,
		},
		"share_url": strings.TrimRight(s.cfg.HostURL, "/") + "/profile/" + strconv.FormatInt(uid, 10),
	})
}

// validateProfileURL 资料里的 URL 字段：允许空；非空必须是 http/https 绝对地址。
func validateProfileURL(v string) bool {
	if v == "" {
		return true
	}
	u, err := url.Parse(v)
	return err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https")
}

package app

import (
	"net/http"
	"time"
)

// 全站统计两层：公开概览给首页，管理端明细面板给后台。
// 时区口径必须跟各数据的写入方一致，否则跨日断言会漂：
//   - submissions/users 的 created_at 是 SQLite CURRENT_TIMESTAMP（UTC），
//     「今日提交/活跃」按 UTC date 聚合；
//   - user_online_days.date 由心跳按 Asia/Shanghai 写入（pointsLocation），
//     「今日在线」按 date('now','+8 hours') 查询。
// 趋势序列用 UTC 日填充，30 天稠密：无数据的日子也返回 0，前端不用补洞。

// statTrendPoint 单日提交趋势。
type statTrendPoint struct {
	Date        string `json:"date"`
	Submissions int    `json:"submissions"`
	Accepted    int    `json:"accepted"`
}

// siteOverview 全站概览计数，两层接口共用。
func (s *Server) siteOverview() (map[string]any, error) {
	counts := map[string]int64{}
	qs := []struct {
		key, sql string
	}{
		{"users", `SELECT COUNT(*) FROM users`},
		{"problems", `SELECT COUNT(*) FROM problems WHERE invisible=0`},
		{"submissions", `SELECT COUNT(*) FROM submissions`},
		{"accepted", `SELECT COUNT(*) FROM submissions WHERE status=?`},
		{"discussions", `SELECT COUNT(*) FROM discussions`},
		{"contests", `SELECT COUNT(*) FROM contests WHERE visible=1`},
		{"plans", `SELECT COUNT(*) FROM training_plans WHERE visible=1`},
		{"notices", `SELECT COUNT(*) FROM notices WHERE visible=1`},
		{"today_submissions", `SELECT COUNT(*) FROM submissions WHERE date(created_at)=date('now')`},
		{"today_accepted", `SELECT COUNT(*) FROM submissions WHERE status=1 AND date(created_at)=date('now')`},
	}
	for _, item := range qs {
		var v int64
		if item.key == "accepted" {
			if err := s.db.QueryRow(item.sql, StatusAccepted).Scan(&v); err != nil {
				return nil, err
			}
		} else if err := s.db.QueryRow(item.sql).Scan(&v); err != nil {
			return nil, err
		}
		counts[item.key] = v
	}
	var todayActive, todayOnline int
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM submissions
		WHERE date(created_at)=date('now')`).Scan(&todayActive); err != nil {
		return nil, err
	}
	// 心跳按 Asia/Shanghai 记日，UTC 加 8 小时对齐同一口径。
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM user_online_days
		WHERE date=date('now','+8 hours')`).Scan(&todayOnline); err != nil {
		return nil, err
	}

	total, acc := counts["submissions"], counts["accepted"]
	return map[string]any{
		"users": counts["users"], "problems": counts["problems"],
		"submissions": total, "accepted": acc,
		"accept_rate":        fmtPct(int(acc), int(total)),
		"discussions":        counts["discussions"],
		"contests":           counts["contests"],
		"plans":              counts["plans"],
		"notices":            counts["notices"],
		"today_submissions":  counts["today_submissions"],
		"today_accepted":     counts["today_accepted"],
		"today_active_users": todayActive,
		"today_online_users": todayOnline,
	}, nil
}

// statsSite 公开全站概览，供首页展示。
func (s *Server) statsSite(w http.ResponseWriter, r *http.Request) {
	ov, err := s.siteOverview()
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, ov)
}

// statsAdmin 管理端统计面板：概览 + 30 天趋势 + 状态/难度分布 + 热题榜 + AC 用户榜。
func (s *Server) statsAdmin(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	ov, err := s.siteOverview()
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 30 天稠密趋势（UTC 日）：先查有数据的天，再按日历补 0。
	trends := make([]statTrendPoint, 30)
	start := time.Now().UTC().AddDate(0, 0, -29)
	for i := range trends {
		trends[i].Date = start.AddDate(0, 0, i).Format("2006-01-02")
	}
	rows, err := s.db.Query(`SELECT date(created_at) AS d, COUNT(*),
		SUM(CASE WHEN status=? THEN 1 ELSE 0 END)
		FROM submissions WHERE created_at >= datetime('now','-29 days')
		GROUP BY d`, StatusAccepted)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	dayIdx := map[string]int{}
	for i, tp := range trends {
		dayIdx[tp.Date] = i
	}
	for rows.Next() {
		var d string
		var total, acc int
		if err := rows.Scan(&d, &total, &acc); err != nil {
			continue
		}
		if i, has := dayIdx[d]; has {
			trends[i].Submissions = total
			trends[i].Accepted = acc
		}
	}
	rows.Close()

	// 状态分布：含等待中；文案复用 StatusText 口径。
	type statKV struct {
		Status int    `json:"status"`
		Text   string `json:"text"`
		Count  int    `json:"count"`
	}
	statusDist := []statKV{}
	if rows, err := s.db.Query(`SELECT status, COUNT(*) FROM submissions GROUP BY status ORDER BY status`); err == nil {
		for rows.Next() {
			var kv statKV
			if err := rows.Scan(&kv.Status, &kv.Count); err != nil {
				continue
			}
			kv.Text = StatusText[kv.Status]
			if kv.Text == "" {
				kv.Text = "等待中"
			}
			statusDist = append(statusDist, kv)
		}
		rows.Close()
	}

	// 难度分布：可见题的题数 + 提交量 + AC 量（JOIN 提交）。
	type diffStat struct {
		Difficulty  string `json:"difficulty"`
		Problems    int    `json:"problems"`
		Submissions int    `json:"submissions"`
		Accepted    int    `json:"accepted"`
	}
	diffMap := map[string]*diffStat{}
	order := []string{}
	if rows, err := s.db.Query(`SELECT difficulty, COUNT(*) FROM problems
		WHERE invisible=0 GROUP BY difficulty ORDER BY difficulty`); err == nil {
		for rows.Next() {
			var d string
			var n int
			if err := rows.Scan(&d, &n); err != nil {
				continue
			}
			diffMap[d] = &diffStat{Difficulty: d, Problems: n}
			order = append(order, d)
		}
		rows.Close()
	}
	if rows, err := s.db.Query(`SELECT p.difficulty, COUNT(s.id),
		SUM(CASE WHEN s.status=? THEN 1 ELSE 0 END)
		FROM submissions s JOIN problems p ON p.id=s.problem_id
		WHERE p.invisible=0 GROUP BY p.difficulty`, StatusAccepted); err == nil {
		for rows.Next() {
			var d string
			var total, acc int
			if err := rows.Scan(&d, &total, &acc); err != nil {
				continue
			}
			ds, has := diffMap[d]
			if !has {
				ds = &diffStat{Difficulty: d}
				diffMap[d] = ds
				order = append(order, d)
			}
			ds.Submissions, ds.Accepted = total, acc
		}
		rows.Close()
	}
	difficulty := []diffStat{}
	for _, d := range order {
		difficulty = append(difficulty, *diffMap[d])
	}

	// 热门题目：按提交量倒序前 10。
	type hotProblem struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Difficulty string `json:"difficulty"`
		Submit     int    `json:"submit"`
		Accept     int    `json:"accept"`
		Rate       string `json:"rate"`
	}
	topProblems := []hotProblem{}
	if rows, err := s.db.Query(`SELECT id, name, difficulty, submit, accept FROM problems
		WHERE invisible=0 ORDER BY submit DESC, id ASC LIMIT 10`); err == nil {
		for rows.Next() {
			var hp hotProblem
			if err := rows.Scan(&hp.ID, &hp.Name, &hp.Difficulty, &hp.Submit, &hp.Accept); err != nil {
				continue
			}
			hp.Rate = fmtPct(hp.Accept, hp.Submit)
			topProblems = append(topProblems, hp)
		}
		rows.Close()
	}

	// AC 用户榜：口径与排行榜一致——去重 AC 题数，附总 AC 提交数。
	type topAuthor struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Solved   int    `json:"solved"`
		Accepted int    `json:"accepted"`
	}
	topAuthors := []topAuthor{}
	if rows, err := s.db.Query(`SELECT s.user_id, u.username,
		COUNT(DISTINCT s.problem_id), COUNT(*)
		FROM submissions s JOIN users u ON u.id=s.user_id
		WHERE s.status=? GROUP BY s.user_id, u.username
		ORDER BY COUNT(DISTINCT s.problem_id) DESC, COUNT(*) ASC LIMIT 10`, StatusAccepted); err == nil {
		for rows.Next() {
			var ta topAuthor
			if err := rows.Scan(&ta.UserID, &ta.Username, &ta.Solved, &ta.Accepted); err != nil {
				continue
			}
			topAuthors = append(topAuthors, ta)
		}
		rows.Close()
	}

	OK(w, map[string]any{
		"overview":     ov,
		"trends":       trends,
		"status_dist":  statusDist,
		"difficulty":   difficulty,
		"top_problems": topProblems,
		"top_authors":  topAuthors,
	})
}

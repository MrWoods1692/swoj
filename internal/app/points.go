package app

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// pointsLocation 用于划分「自然日」。优先 Asia/Shanghai，缺失时回落本地时区。
var pointsLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.Local
}()

// PointsCategory 积分流水的业务来源。
const (
	CategoryCheckin = "checkin"
	CategoryAC      = "ac"
	CategoryOnline  = "online"
	CategoryContest = "contest"
	CategoryAdmin   = "admin"
	CategoryShop    = "shop"
)

// signPoints 计算第 streak 天的签到积分：第 1 天给 base，之后每天 +bonus，上限 max。
func (c *PointsConfig) signPoints(streak int) int {
	if streak < 1 {
		streak = 1
	}
	p := c.SignBase + (streak-1)*c.SignBonus
	if p > c.SignMax {
		return c.SignMax
	}
	return p
}

// acPoints 返回题目难度对应的单次 AC 积分。
func (c *PointsConfig) acPoints(difficulty string) int {
	if n, ok := c.AC[strings.TrimSpace(difficulty)]; ok && n > 0 {
		return n
	}
	return c.ACDefault
}

// onlinePointsPerHours 换算每小时在线积分，避免除零。
func (c *PointsConfig) onlineUnitSeconds() int {
	if c.OnlineHours < 1 {
		return 3600
	}
	return c.OnlineHours * 3600
}

// awardPoints 给单个用户增减积分并落一条流水。
// 注意：数据库为单连接，禁止在此开事务（Begin 会占住唯一连接导致后续写死锁），
// 必须用顺序写入。返回实际入账积分；delta 为 0 时不做任何写入。
func (s *Server) awardPoints(userID int64, delta int, category, refType string, refID int64, remark string) (int, error) {
	if delta == 0 {
		return 0, nil
	}
	if _, err := s.db.Exec(`UPDATE users SET points = points + ? WHERE id=?`, delta, userID); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, userID, delta, category, refType, refID, remark); err != nil {
		return 0, err
	}
	return delta, nil
}

// CheckinResult 签到返回体。
type CheckinResult struct {
	CheckedIn bool   `json:"checked_in"`
	Today     string `json:"date"`
	Streak    int    `json:"streak"`
	Points    int    `json:"points"`
	Balance   int    `json:"balance"`
}

// doCheckin 处理一次签到：当日重复签到返回已有结果，不重复发分。
func (s *Server) doCheckin(userID int64) (*CheckinResult, error) {
	today := time.Now().In(pointsLocation).Format("2006-01-02")

	// 先查当日是否已签：已签则直接回现有结果，不重复发分。
	var existingStreak, existingPoints, balance, streak, points int
	var err error
	if err = s.db.QueryRow(`SELECT streak, points FROM checkins WHERE user_id=? AND date=?`, userID, today).
		Scan(&existingStreak, &existingPoints); err == nil {
		_ = s.db.QueryRow(`SELECT points FROM users WHERE id=?`, userID).Scan(&balance)
		return &CheckinResult{CheckedIn: true, Today: today, Streak: existingStreak,
			Points: existingPoints, Balance: balance}, nil
	}

	yesterday := time.Now().In(pointsLocation).AddDate(0, 0, -1).Format("2006-01-02")
	if err = s.db.QueryRow(`SELECT streak FROM checkins WHERE user_id=? AND date=?`,
		userID, yesterday).Scan(&streak); err == nil {
		streak++
	} else {
		streak = 1
	}
	points = s.cfg.Points.signPoints(streak)

	// 靠 UNIQUE(user_id, date) 保证当日只写一次；RowsAffected==0 说明并发下已被
	// 其他请求写入，此时不发分，只回现有记录。
	res, err := s.db.Exec(`INSERT INTO checkins(user_id, date, streak, points) VALUES(?,?,?,?)
		ON CONFLICT(user_id, date) DO NOTHING`, userID, today, streak, points)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_ = s.db.QueryRow(`SELECT streak, points FROM checkins WHERE user_id=? AND date=?`, userID, today).
			Scan(&streak, &points)
	} else {
		if _, err := s.db.Exec(`UPDATE users SET points = points + ? WHERE id=?`, points, userID); err != nil {
			return nil, err
		}
		if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
			VALUES(?,?,?,?,?,?)`, userID, points, CategoryCheckin, "checkin", 0,
			fmt.Sprintf("连续签到第 %d 天", streak)); err != nil {
			return nil, err
		}
	}
	balance, err = s.userPoints(userID)
	if err != nil {
		return nil, err
	}
	return &CheckinResult{CheckedIn: true, Today: today, Streak: streak,
		Points: points, Balance: balance}, nil
}

// userPoints 读取当前积分余额。
func (s *Server) userPoints(userID int64) (int, error) {
	var p int
	return p, s.db.QueryRow(`SELECT points FROM users WHERE id=?`, userID).Scan(&p)
}

// recordOnline 累计在线时长，并在每满一档时发放积分。返回本次新增积分。
func (s *Server) recordOnline(userID int64, seconds int) (int, error) {
	if seconds <= 0 {
		return 0, nil
	}
	unit := s.cfg.Points.onlineUnitSeconds()

	// 发放以「累计满档数 - 已发放数」为据，重复上报幂等；
	// 多连接 + WAL 下单条 UPDATE 本身是原子的，不需要包裹事务。
	if _, err := s.db.Exec(`INSERT INTO online_stats(user_id, online_seconds) VALUES(?,?)
		ON CONFLICT(user_id) DO UPDATE SET online_seconds = online_seconds + excluded.online_seconds,
		updated_at = CURRENT_TIMESTAMP`, userID, seconds); err != nil {
		return 0, err
	}
	var total, awarded int
	if err := s.db.QueryRow(`SELECT online_seconds, points_awarded FROM online_stats WHERE user_id=?`, userID).
		Scan(&total, &awarded); err != nil {
		return 0, err
	}
	eligible := total / unit
	gain := (eligible - awarded) * s.cfg.Points.OnlinePoints
	if gain <= 0 {
		return 0, nil
	}
	if _, err := s.db.Exec(`UPDATE online_stats SET points_awarded=? WHERE user_id=?`, eligible, userID); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`UPDATE users SET points = points + ? WHERE id=?`, gain, userID); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, userID, gain, CategoryOnline, "online", userID,
		fmt.Sprintf("在线满 %d 小时", s.cfg.Points.OnlineHours*gain)); err != nil {
		return 0, err
	}
	return gain, nil
}

// PointsLogRow 积分流水。
type PointsLogRow struct {
	ID        int64  `json:"id"`
	Delta     int    `json:"delta"`
	Category  string `json:"category"`
	RefType   string `json:"ref_type"`
	RefID     int64  `json:"ref_id"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"created_at"`
}

// pointsLog 返回积分流水，category 为空表示全部。
func (s *Server) pointsLog(userID int64, category string, limit int) ([]PointsLogRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `SELECT id, delta, category, ref_type, ref_id, remark, created_at FROM points_log WHERE user_id=?`
	args := []any{userID}
	if category != "" {
		q += ` AND category=?`
		args = append(args, category)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PointsLogRow{}
	for rows.Next() {
		var it PointsLogRow
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Delta, &it.Category, &it.RefType, &it.RefID, &it.Remark,
			(*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		list = append(list, it)
	}
	return list, nil
}

// PointsRank 积分排行榜条目。
type PointsRank struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	RealName string `json:"realname"`
	School   string `json:"school"`
	Points   int    `json:"points"`
}

// pointsLeaderboard 按积分倒序返回排行榜。
func (s *Server) pointsLeaderboard(limit int) ([]PointsRank, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT username, realname, school, points FROM users
		WHERE role NOT IN ('admin','super','superadmin') ORDER BY points DESC, id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PointsRank{}
	i := 1
	for rows.Next() {
		var it PointsRank
		if err := rows.Scan(&it.Username, &it.RealName, &it.School, &it.Points); err != nil {
			continue
		}
		it.Rank = i
		i++
		list = append(list, it)
	}
	return list, nil
}

// contestRankKeys 把名次规则表按数字升序返回，便于逐名次匹配。
func contestRankKeys(m map[string]int) []int {
	keys := []int{}
	for k := range m {
		n := 0
		if _, err := fmt.Sscanf(k, "%d", &n); err == nil && n > 0 {
			keys = append(keys, n)
		}
	}
	sort.Ints(keys)
	return keys
}

// checkinStreak 返回当前连续签到天数：今日已签则返回今日链长，
// 昨日已签但未签今日返回昨日+1，否则返回 0。
// SQLite 的 CURRENT_TIMESTAMP 存 UTC，需用 localtime 与本地日期对齐。
func (s *Server) checkinStreak(userID int64) int {
	today := time.Now().In(pointsLocation).Format("2006-01-02")
	var streak int
	if err := s.db.QueryRow(`SELECT streak FROM checkins WHERE user_id=? AND date=?`, userID, today).Scan(&streak); err == nil {
		return streak
	}
	var y int
	if err := s.db.QueryRow(`SELECT streak FROM checkins WHERE user_id=?
		AND date = date(date('now', 'localtime'), '-1 day')`, userID).Scan(&y); err == nil {
		return y + 1
	}
	return 0
}

package app

import (
	"net/http"
	"time"
)

// checkin 手动签到，每日一次。
func (s *Server) checkin(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	res, err := s.doCheckin(claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.refreshAchievements(claims.UserID)
	OK(w, res)
}

// checkinStatus 返回今日签到状态与连续天数，未签到不改变任何数据。
func (s *Server) checkinStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	today := time.Now().In(pointsLocation).Format("2006-01-02")
	var res CheckinResult
	res.Today = today
	res.Streak = s.checkinStreak(claims.UserID)
	if err := s.db.QueryRow(`SELECT streak, points FROM checkins WHERE user_id=? AND date=?`,
		claims.UserID, today).Scan(&res.Streak, &res.Points); err == nil {
		res.CheckedIn = true
	} else {
		res.Streak = s.checkinStreak(claims.UserID)
	}
	res.Balance, _ = s.userPoints(claims.UserID)
	OK(w, res)
}

// onlineHeartbeat 上报在线时长，服务端据此发放满档积分。
// 前端周期性调用；单次上报超过上限会被截断，避免客户端伪造大额时长。
func (s *Server) onlineHeartbeat(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Seconds int `json:"seconds"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Seconds > 600 {
		req.Seconds = 600
	}
	gain, err := s.recordOnline(claims.UserID, req.Seconds)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.refreshAchievements(claims.UserID)
	balance, _ := s.userPoints(claims.UserID)
	OK(w, map[string]any{"seconds": req.Seconds, "points": gain, "balance": balance})
}

// onlineSummary 返回在线累计与已发放积分。
func (s *Server) onlineSummary(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var seconds, awarded, balance int
	_ = s.db.QueryRow(`SELECT online_seconds, points_awarded FROM online_stats WHERE user_id=?`,
		claims.UserID).Scan(&seconds, &awarded)
	_ = s.db.QueryRow(`SELECT points FROM users WHERE id=?`, claims.UserID).Scan(&balance)
	OK(w, map[string]any{
		"online_seconds": seconds,
		"hours":          seconds / 3600,
		"points_awarded": awarded * s.cfg.Points.OnlinePoints,
		"next_point_at":  (awarded + 1) * s.cfg.Points.onlineUnitSeconds(),
		"balance":        balance,
	})
}

// pointsDetail 返回我的积分余额、流水与基础规则。
func (s *Server) pointsDetail(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	balance, _ := s.userPoints(claims.UserID)
	logs, err := s.pointsLog(claims.UserID, r.URL.Query().Get("category"),
		atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{
		"balance": balance,
		"streak":  s.checkinStreak(claims.UserID),
		"log":     logs,
		"rules":   s.pointsRules(),
	})
}

// pointsRules 返回当前生效的积分规则内容。
func (s *Server) pointsRules() map[string]any {
	c := s.cfg.Points
	if c == nil {
		c = DefaultPointsConfig()
	}
	return map[string]any{
		"sign_base":       c.SignBase,
		"sign_bonus":      c.SignBonus,
		"sign_max":        c.SignMax,
		"ac":              c.AC,
		"ac_default":      c.ACDefault,
		"online_hours":    c.OnlineHours,
		"online_points":   c.OnlinePoints,
		"contest":         c.Contest,
		"contest_default": c.ContestDefault,
	}
}

// pointsRulesHandler 公开返回积分规则，供前端展示说明。
func (s *Server) pointsRulesHandler(w http.ResponseWriter, r *http.Request) {
	OK(w, s.pointsRules())
}

// pointsLeaderboardHandler 积分排行榜，公开访问。
func (s *Server) pointsLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	list, err := s.pointsLeaderboard(limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"list": list, "limit": limit})
}

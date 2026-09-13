package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LevelTier 等级档位。Min 仅作展示；等级由积分购买升级决定，不随积分自动变化。
type LevelTier struct {
	Level int    `json:"level"`
	Name  string `json:"name"`
	Min   int    `json:"min"`
}

// DefaultLevelTiers 默认等级表：入门到传说共 10 级。
func DefaultLevelTiers() []LevelTier {
	return []LevelTier{
		{1, "入门", 0}, {2, "初级", 30}, {3, "中级", 80}, {4, "高级", 180},
		{5, "专家", 350}, {6, "大师", 600}, {7, "宗师", 1000}, {8, "强者", 1600},
		{9, "传奇", 2400}, {10, "传说", 3500},
	}
}

// defaultLevelTiers 返回默认等级表副本，避免调用方误改全局值。
func defaultLevelTiers() []LevelTier {
	src := DefaultLevelTiers()
	out := make([]LevelTier, len(src))
	copy(out, src)
	return out
}

// levelCost 升到 target 级所需积分。每级递增，避免高等级可低价买满。
func levelCost(targetLevel int) int {
	if targetLevel <= 1 || targetLevel > 10 {
		return 0
	}
	return 50 + (targetLevel-2)*30
}

// levelCostRange 累计从 cur 升到 target 的总花费。
func levelCostRange(cur, target int) int {
	total := 0
	for l := cur + 1; l <= target; l++ {
		total += levelCost(l)
	}
	return total
}

// LevelInfo 等级视图：当前档、名称、升到下一级的费用。
type LevelInfo struct {
	Level      int    `json:"level"`
	Name       string `json:"name"`
	Min        int    `json:"min"`
	CostToNext int    `json:"cost_to_next"`
	MaxLevel   bool   `json:"max_level"`
}

// levelInfoOf 按已购买等级查档。等级是账号属性，不是积分派生值。
func levelInfoOf(tiers []LevelTier, cur int) *LevelInfo {
	if len(tiers) == 0 {
		tiers = defaultLevelTiers()
	}
	curTier := tiers[0]
	for _, t := range tiers {
		if t.Level == cur {
			curTier = t
			break
		}
	}
	li := &LevelInfo{Level: curTier.Level, Name: curTier.Name, Min: curTier.Min}
	if curTier.Level >= len(tiers) {
		li.MaxLevel = true
	} else {
		li.CostToNext = levelCost(curTier.Level + 1)
	}
	return li
}

// achStats 聚合成就判定所需的全部统计。一次查齐，避免逐成就打库。
type achStats struct {
	Accepted       int
	WA             int
	Discussions    int
	MaxLoginStreak int
	MaxACStreak    int
	MaxDailyOnline int
	TrainingDone   bool
	RegSeconds     int
	ProfileFull    bool
}

// AchievementDef 成就定义：静态描述 + 判定函数。
// 新增成就只需在 AchievementList 追加，无需改表结构。
type AchievementDef struct {
	Code   string
	Name   string
	Desc   string
	Icon   string
	Unlock func(*achStats) bool
}

// AchievementList 全部成就定义。
var AchievementList = []AchievementDef{
	{Code: "first_problem", Name: "初来乍到", Desc: "首次提交任意题目（AC 或 WA）", Icon: "📝",
		Unlock: func(s *achStats) bool { return s.Accepted >= 1 || s.WA >= 1 }},
	{Code: "first_ac", Name: "初见曙光", Desc: "首次获得 AC", Icon: "🎉",
		Unlock: func(s *achStats) bool { return s.Accepted >= 1 }},
	{Code: "ac_1k", Name: "千题俱乐部", Desc: "累计获得 1000 次 AC", Icon: "💪",
		Unlock: func(s *achStats) bool { return s.Accepted >= 1000 }},
	{Code: "ac_10k", Name: "千锤百炼", Desc: "累计获得 10000 次 AC", Icon: "🏆",
		Unlock: func(s *achStats) bool { return s.Accepted >= 10000 }},
	{Code: "first_wa", Name: "初尝挫折", Desc: "第一次提交 WA", Icon: "😅",
		Unlock: func(s *achStats) bool { return s.WA >= 1 }},
	{Code: "wa_1k", Name: "WA 常客", Desc: "累计提交 1000 次 WA", Icon: "😢",
		Unlock: func(s *achStats) bool { return s.WA >= 1000 }},
	{Code: "wa_10k", Name: "WA 大师", Desc: "累计提交 10000 次 WA", Icon: "💀",
		Unlock: func(s *achStats) bool { return s.WA >= 10000 }},
	{Code: "first_discussion", Name: "崭露头角", Desc: "发布第一条讨论帖", Icon: "💬",
		Unlock: func(s *achStats) bool { return s.Discussions >= 1 }},
	{Code: "discussion_1k", Name: "活跃讨论者", Desc: "累计发布 1000 条讨论帖", Icon: "📢",
		Unlock: func(s *achStats) bool { return s.Discussions >= 1000 }},
	{Code: "discussion_10k", Name: "话痨大师", Desc: "累计发布 10000 条讨论帖", Icon: "📚",
		Unlock: func(s *achStats) bool { return s.Discussions >= 10000 }},
	{Code: "register_1y", Name: "一年之约", Desc: "注册满 1 年", Icon: "🎂",
		Unlock: func(s *achStats) bool { return s.RegSeconds >= 365*24*3600 }},
	{Code: "login_streak_7", Name: "七日之约", Desc: "连续登录 7 天", Icon: "📅",
		Unlock: func(s *achStats) bool { return s.MaxLoginStreak >= 7 }},
	{Code: "login_streak_30", Name: "月度坚持", Desc: "连续登录 30 天", Icon: "🏅",
		Unlock: func(s *achStats) bool { return s.MaxLoginStreak >= 30 }},
	{Code: "ac_streak_7", Name: "七日连击", Desc: "连续 7 天每天至少 AC 一次", Icon: "🔥",
		Unlock: func(s *achStats) bool { return s.MaxACStreak >= 7 }},
	{Code: "online_6h", Name: "专注六小时", Desc: "单日累计在线满 6 小时", Icon: "⏳",
		Unlock: func(s *achStats) bool { return s.MaxDailyOnline >= 6*3600 }},
	{Code: "training_done", Name: "完成训练计划", Desc: "完成一次训练计划单", Icon: "✅",
		Unlock: func(s *achStats) bool { return s.TrainingDone }},
	{Code: "profile_full", Name: "个性主页", Desc: "完善个人主页（真实姓名、QQ 号、简介）", Icon: "🌟",
		Unlock: func(s *achStats) bool { return s.ProfileFull }},
}

// AchievementItem 成就列表项。
type AchievementItem struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Icon       string `json:"icon"`
	Unlocked   bool   `json:"unlocked"`
	UnlockedAt string `json:"unlocked_at"`
}

// dbAchStats 聚合单个用户的成就统计。接受 *DB 以便 judge 等非 Server 上下文复用。
func dbAchStats(db *DB, userID int64) (*achStats, error) {
	st := &achStats{}
	if err := db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=? AND status=?`,
		userID, StatusAccepted).Scan(&st.Accepted); err != nil {
		return nil, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=?`, userID).Scan(new(int)); err != nil {
		return nil, err
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=? AND status=?`,
		userID, StatusWrongAns).Scan(&st.WA)
	_ = db.QueryRow(`SELECT COUNT(*) FROM discussions WHERE user_id=?`, userID).Scan(&st.Discussions)

	// 登录连续天数：按 checkins.date 计算最长连续。
	st.MaxLoginStreak = maxDateStreak(db,
		fmt.Sprintf(`SELECT date FROM checkins WHERE user_id=%d ORDER BY date`, userID))

	// 连续刷题天数：按 AC 提交日期计算最长连续。
	st.MaxACStreak = maxACStreak(db, userID)

	// 单日最长在线时长。
	_ = db.QueryRow(`SELECT COALESCE(MAX(online_seconds),0) FROM user_online_days WHERE user_id=?`,
		userID).Scan(&st.MaxDailyOnline)

	// 完成训练计划：用户创建的计划中，至少一个计划的记录全部通过。
	var planID int64
	if err := db.QueryRow(`SELECT id FROM training_plans WHERE creator=?`, userID).Scan(&planID); err == nil {
		var planTotal, planAccepted int
		_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(accepted),0) FROM training_records WHERE plan_id=?`, planID).
			Scan(&planTotal, &planAccepted)
		st.TrainingDone = planTotal > 0 && planTotal == planAccepted
	}

	var createdAt time.Time
	if err := db.QueryRow(`SELECT created_at FROM users WHERE id=?`, userID).Scan((*time.Time)(&createdAt)); err == nil {
		st.RegSeconds = int(time.Since(createdAt).Seconds())
	}

	var realname, qq, signature string
	_ = db.QueryRow(`SELECT COALESCE(realname,''), COALESCE(qq,''), COALESCE(signature,'') FROM users WHERE id=?`,
		userID).Scan(&realname, &qq, &signature)
	// 头像禁止手动填写、改由 QQ 自动派生，因此「头像已设置」等价于 QQ 号有效。
	st.ProfileFull = strings.TrimSpace(realname) != "" && qqAvatarURL(qq) != "" &&
		strings.TrimSpace(signature) != ""
	return st, nil
}

// maxDateStreak 按已排序的日期序列算最长连续天数；SQL 侧已排序。
func maxDateStreak(db *DB, query string) int {
	rows, err := db.Query(query)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var prev string
	cur, best := 0, 0
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			continue
		}
		if prev == "" {
			cur = 1
		} else if daysBetween(prev, d) == 1 {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
		prev = d
	}
	return best
}

// maxACStreak 按 AC 提交日期算最长连续天数。
func maxACStreak(db *DB, userID int64) int {
	rows, err := db.Query(`SELECT DISTINCT date(created_at) AS d
		FROM submissions WHERE user_id=? AND status=? ORDER BY d`, userID, StatusAccepted)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var prev string
	cur, best := 0, 0
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			continue
		}
		if prev == "" {
			cur = 1
		} else if daysBetween(prev, d) == 1 {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
		prev = d
	}
	return best
}

// daysBetween 返回 b-a 的自然日差，用于判定连续性。
func daysBetween(a, b string) int {
	ta, ea := time.ParseInLocation("2006-01-02", a, pointsLocation)
	tb, eb := time.ParseInLocation("2006-01-02", b, pointsLocation)
	if ea != nil || eb != nil {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// dbRefreshAchievements 重新评估用户成就并写入新解锁项。幂等：已解锁的跳过。
// 独立函数（非 Server 方法），供 judge 等非 Server 上下文调用。
func dbRefreshAchievements(db *DB, userID int64) ([]string, error) {
	st, err := dbAchStats(db, userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT code FROM user_achievements WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	got := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			continue
		}
		got[c] = true
	}
	rows.Close()

	var out []string = []string{}
	for _, d := range AchievementList {
		if got[d.Code] || !d.Unlock(st) {
			continue
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO user_achievements(user_id, code) VALUES(?,?)`,
			userID, d.Code); err != nil {
			return out, err
		}
		out = append(out, d.Code)
	}
	return out, nil
}

// userLevelOf 读取用户当前已购买等级，缺省为 1。
func (s *Server) userLevelOf(userID int64) int {
	var lv int
	_ = s.db.QueryRow(`SELECT level FROM users WHERE id=?`, userID).Scan(&lv)
	if lv <= 0 {
		lv = 1
	}
	return lv
}

// achStats 便捷包装。
func (s *Server) achStats(userID int64) (*achStats, error) {
	return dbAchStats(s.db, userID)
}

// refreshAchievements 便捷包装，忽略错误。
func (s *Server) refreshAchievements(userID int64) {
	_, _ = dbRefreshAchievements(s.db, userID)
}

// recordOnlineDay 把一次在线心跳累加到当日记录，供「连续在线 6 小时」成就判定。
// 与积分用的 online_stats 独立，专门保留每日粒度的时长。
func (s *Server) recordOnlineDay(userID int64, seconds int) {
	if seconds <= 0 {
		return
	}
	today := time.Now().In(pointsLocation).Format("2006-01-02")
	_, _ = s.db.Exec(`INSERT INTO user_online_days(user_id, date, online_seconds) VALUES(?,?,?)
		ON CONFLICT(user_id, date) DO UPDATE SET online_seconds = online_seconds + excluded.online_seconds`,
		userID, today, seconds)
}

// profileUpdate 更新自己的个人资料：真实姓名、学校、头像、简介。
// 字段部分提交也可，未传项保持不变；管理员无法通过此接口改他人资料。
// 真实姓名仅可首次补填，此后只能由管理员改；QQ 号是校园墙 OAuth 身份键
// （oauth_id 同值），不允许本人覆盖，否则会把账号绑到无关身份上且无法反向找回。
func (s *Server) profileUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		RealName   *string `json:"realname"`
		School     *string `json:"school"`
		Avatar     *string `json:"avatar"`
		Signature  *string `json:"signature"`
		Website    *string `json:"website"`
		Background *string `json:"background"`
		QQ         *string `json:"qq"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	// 部分更新：只写前端传了的字段，避免分步补资料时清空已填内容。
	sets := []string{}
	args := []any{}
	if req.RealName != nil {
		name := strings.TrimSpace(*req.RealName)
		// 真实姓名仅允许首次补填：建号时 OAuth 不写入该列，允许本人反复改名
		// 会破坏展示名与学籍的对应关系，故已有值后只可由管理员改。
		// 此判定先于格式校验：否则已有姓名的用户改名为 2 字时会被格式错误抢先
		// 拦下，报出误导性文案且本分支永不触发。
		var cur string
		if err := s.db.QueryRow(`SELECT COALESCE(realname,'') FROM users WHERE id=?`,
			claims.UserID).Scan(&cur); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if cur != "" {
			// 清空也算修改：否则用户先清空再重填，就能绕过“仅一次”的限制。
			Fail(w, http.StatusForbidden, "真实姓名已填写，如需修改请联系管理员")
			return
		}
		if name == "" || !cnRealname.MatchString(name) {
			Fail(w, http.StatusBadRequest, "真实姓名需为 3-4 个汉字")
			return
		}
		sets = append(sets, "realname=?")
		args = append(args, name)
	}
	if req.School != nil {
		sets = append(sets, "school=?")
		args = append(args, strings.TrimSpace(*req.School))
	}
	// QQ 号由 OAuth 建号时写入，同时作为 oauth_id 的身份键；
	// 允许本人覆盖会破坏该绑定，且被覆盖后无法按 OAuth 回查找回原账号。
	if req.QQ != nil {
		Fail(w, http.StatusBadRequest, "QQ 号由校园墙授权写入，不可修改")
		return
	}
	// 头像由平台按 QQ 自动派生（见 handler_profile.go 的 qqAvatarURL 回退），
	// 不允许用户手动覆盖，否则会出现外链失效或指向无关图片。
	if req.Avatar != nil {
		Fail(w, http.StatusBadRequest, "头像由系统自动生成，不可修改")
		return
	}
	if req.Signature != nil {
		v := strings.TrimSpace(*req.Signature)
		if len([]rune(v)) > 50 {
			Fail(w, http.StatusBadRequest, "个人简介最多 50 个字")
			return
		}
		sets = append(sets, "signature=?")
		args = append(args, v)
	}
	if req.Website != nil {
		v := strings.TrimSpace(*req.Website)
		if !validateProfileURL(v) {
			Fail(w, http.StatusBadRequest, "个人网站需为 http/https 完整链接")
			return
		}
		sets = append(sets, "website=?")
		args = append(args, v)
	}
	if req.Background != nil {
		v := strings.TrimSpace(*req.Background)
		if !validateProfileURL(v) {
			Fail(w, http.StatusBadRequest, "背景图需为 http/https 完整链接")
			return
		}
		sets = append(sets, "background=?")
		args = append(args, v)
	}
	if req.QQ != nil {
		v := strings.TrimSpace(*req.QQ)
		if v != "" && !qqNumber.MatchString(v) {
			Fail(w, http.StatusBadRequest, "QQ 号需为 5-11 位数字")
			return
		}
		sets = append(sets, "qq=?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		Fail(w, http.StatusBadRequest, "没有需要更新的字段")
		return
	}
	args = append(args, claims.UserID)
	_, err := s.db.Exec(`UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id=?`, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 姓名与个人主页补全后，才算完成注册：解锁提交权限。
	var realname, qq, signature string
	if err := s.db.QueryRow(`SELECT COALESCE(realname,''), COALESCE(qq,''), COALESCE(signature,'')
		FROM users WHERE id=?`, claims.UserID).Scan(&realname, &qq, &signature); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 头像由 QQ 派生，判定时改用 QQ 有效性，否则禁止改头像后提交权限永远无法解锁。
	if realname != "" && qqAvatarURL(qq) != "" && strings.TrimSpace(signature) != "" {
		_, _ = s.db.Exec(`UPDATE users SET can_submit=1 WHERE id=?`, claims.UserID)
	}
	s.refreshAchievements(claims.UserID)
	u, _ := s.userByID(claims.UserID)
	OK(w, u)
}

// achievements 返回全部成就定义、解锁状态与本次新解锁项。
func (s *Server) achievements(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	uid := claims.UserID
	newly, _ := dbRefreshAchievements(s.db, uid)

	rows, err := s.db.Query(`SELECT code, unlocked_at FROM user_achievements WHERE user_id=?`, uid)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	unlocked := map[string]string{}
	for rows.Next() {
		var c string
		var at time.Time
		if err := rows.Scan(&c, (*time.Time)(&at)); err != nil {
			continue
		}
		unlocked[c] = at.String()
	}
	rows.Close()

	list := make([]AchievementItem, 0, len(AchievementList))
	for _, d := range AchievementList {
		it := AchievementItem{Code: d.Code, Name: d.Name, Desc: d.Desc, Icon: d.Icon}
		if at, has := unlocked[d.Code]; has {
			it.Unlocked = true
			it.UnlockedAt = at
		}
		list = append(list, it)
	}
	st, _ := s.achStats(uid)
	OK(w, map[string]any{
		"list":     list,
		"unlocked": len(unlocked),
		"total":    len(AchievementList),
		"new":      newly,
		"stats":    map[string]any{"accepted": st.Accepted, "wa": st.WA, "discussions": st.Discussions},
	})
}

// levelTable 公开返回等级档位表，供前端渲染等级体系。
func (s *Server) levelTable(w http.ResponseWriter, r *http.Request) {
	tiers := s.cfg.Levels
	if len(tiers) == 0 {
		tiers = defaultLevelTiers()
	}
	OK(w, map[string]any{"tiers": tiers})
}

// levelMe 返回当前用户等级与积分。
func (s *Server) levelMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	bal, _ := s.userPoints(claims.UserID)
	li := levelInfoOf(s.cfg.Levels, s.userLevelOf(claims.UserID))
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM user_achievements WHERE user_id=?`, claims.UserID).Scan(&count)
	OK(w, map[string]any{
		"points":       bal,
		"level":        li,
		"cost_to_next": li.CostToNext,
		"achievements": count,
	})
}

// levelBuy 用积分购买等级升级。支持一次升多级，累计计算费用。
func (s *Server) levelBuy(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Levels int `json:"levels"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Levels <= 0 {
		req.Levels = 1
	}
	cur := s.userLevelOf(claims.UserID)
	if cur <= 0 {
		cur = 1
	}
	tiers := s.cfg.Levels
	if len(tiers) == 0 {
		tiers = defaultLevelTiers()
	}
	if cur >= len(tiers) {
		Fail(w, http.StatusConflict, "已达最高等级")
		return
	}
	if cur+req.Levels > len(tiers) {
		req.Levels = len(tiers) - cur
	}
	target := cur + req.Levels
	cost := levelCostRange(cur, target)
	if cost <= 0 {
		Fail(w, http.StatusBadRequest, "无法升级")
		return
	}
	bal, _ := s.userPoints(claims.UserID)
	if bal < cost {
		Failf(w, http.StatusPaymentRequired, "积分不足，升级 %d 级需要 %d 分，当前 %d 分", req.Levels, cost, bal)
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET points = points - ?, level = ? WHERE id=?`,
		cost, target, claims.UserID); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, claims.UserID, -cost, CategoryLevelBuy, "level", int64(target),
		fmt.Sprintf("升级至 Lv.%d -%d", target, cost)); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	bal, _ = s.userPoints(claims.UserID)
	li := levelInfoOf(tiers, target)
	s.refreshAchievements(claims.UserID)
	OK(w, map[string]any{
		"level":        target,
		"points":       bal,
		"spend":        cost,
		"cost_to_next": li.CostToNext,
		"max_level":    li.MaxLevel,
	})
}

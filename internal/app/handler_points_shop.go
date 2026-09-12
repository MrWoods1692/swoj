package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// contestPointsSetRule 管理员为某场比赛设定名次积分规则。
func (s *Server) contestPointsSetRule(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	var req struct {
		Rule map[string]int `json:"rule"` // 名次(字符串) -> 积分
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Rule) == 0 {
		Fail(w, http.StatusBadRequest, "名次规则不能为空")
		return
	}
	if _, err := s.db.Exec(`INSERT INTO contest_points_rules(contest_id, rule) VALUES(?,?)
		ON CONFLICT(contest_id) DO UPDATE SET rule=excluded.rule, updated_at=CURRENT_TIMESTAMP`,
		id, contestRuleJSON(req.Rule)); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "contest_points_rule", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"contest_id": id, "rule": req.Rule})
}

// contestPointsGetRule 查询比赛名次积分规则。
func (s *Server) contestPointsGetRule(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	var raw string
	err := s.db.QueryRow(`SELECT rule FROM contest_points_rules WHERE contest_id=?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		OK(w, map[string]any{"contest_id": id, "rule": nil})
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"contest_id": id, "rule": contestRuleToMap(raw)})
}

// contestPointsApply 按当前名次规则为比赛发放积分，每场每人只发一次。
func (s *Server) contestPointsApply(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	raw := ""
	_ = s.db.QueryRow(`SELECT rule FROM contest_points_rules WHERE contest_id=?`, id).Scan(&raw)
	if strings.TrimSpace(raw) == "" {
		Fail(w, http.StatusBadRequest, "该比赛未设置名次积分规则")
		return
	}
	rule := contestRuleToMap(raw)
	if len(rule) == 0 {
		Fail(w, http.StatusBadRequest, "名次规则为空")
		return
	}

	// 名次口径与排行榜一致：按通过题数倒序，同分按首次通过时间正序。
	rows, err := s.db.Query(`SELECT username, COUNT(DISTINCT problem_id) AS acc, MIN(created_at)
		FROM submissions WHERE contest_id=? AND status=? GROUP BY username
		ORDER BY acc DESC, MIN(created_at) ASC`, id, StatusAccepted)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	granted := 0
	rank := 0
	for rows.Next() {
		var uname string
		var acc int
		var first time.Time
		if err := rows.Scan(&uname, &acc, (*time.Time)(&first)); err != nil {
			continue
		}
		rank++
		gain, ok := rule[strconv.Itoa(rank)]
		if !ok {
			continue
		}
		if s.grantContestPoints(id, uname, rank, gain) {
			granted++
		}
	}
	logOp(s.db, claims, "contest_points_apply", strconv.FormatInt(id, 10),
		"granted="+strconv.Itoa(granted), clientIP(r))
	OK(w, map[string]any{"contest_id": id, "granted": granted})
}

// grantContestPoints 给用户在某场比赛发放名次积分，同一场不重复。
func (s *Server) grantContestPoints(contestID int64, username string, rank, gain int) bool {
	if gain <= 0 {
		return false
	}
	var uid int64
	if err := s.db.QueryRow(`SELECT id FROM users WHERE username=?`, username).Scan(&uid); err != nil {
		return false
	}
	var done int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM points_log WHERE user_id=? AND category=?
		AND ref_type=? AND ref_id=?`, uid, CategoryContest, "contest", contestID).Scan(&done)
	if done > 0 {
		return false
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE users SET points = points + ? WHERE id=?`, gain, uid); err != nil {
		return false
	}
	if _, err := tx.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, uid, gain, CategoryContest, "contest", contestID,
		"比赛排名第 "+strconv.Itoa(rank)+" 名"); err != nil {
		return false
	}
	return tx.Commit() == nil
}

// contestRuleJSON 把名次规则序列化为紧凑 JSON 字符串。
func contestRuleJSON(m map[string]int) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// contestRuleToMap 把数据库中的规则 JSON 还原为名次->积分。
func contestRuleToMap(raw string) map[string]int {
	m := map[string]int{}
	if strings.TrimSpace(raw) == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

// ShopItemResp 积分商城商品。
type ShopItemResp struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int    `json:"price"`
	Stock       int    `json:"stock"`
	Status      int    `json:"status"`
	Publisher   string `json:"publisher"`
	CreatedAt   string `json:"created_at"`
}

// shopList 积分商城商品列表，仅显示上架且有库存的商品。
func (s *Server) shopList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT i.id, i.name, i.description, i.price, i.stock, i.status,
		COALESCE(u.username, '') FROM shop_items i LEFT JOIN users u ON u.id=i.publisher_id
		WHERE i.status=1 AND i.stock>0 ORDER BY i.id DESC LIMIT 200`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ShopItemResp{}
	for rows.Next() {
		var it ShopItemResp
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &it.Price, &it.Stock,
			&it.Status, &it.Publisher); err != nil {
			continue
		}
		list = append(list, it)
	}
	OK(w, list)
}

// shopCreate 教师/管理员发布积分商品。
func (s *Server) shopCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Price       int    `json:"price"`
		Stock       int    `json:"stock"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		Fail(w, http.StatusBadRequest, "商品名称不能为空")
		return
	}
	if req.Price < 0 {
		Fail(w, http.StatusBadRequest, "价格不能为负数")
		return
	}
	if req.Stock < 1 {
		Fail(w, http.StatusBadRequest, "库存至少为 1")
		return
	}
	res, err := s.db.Exec(`INSERT INTO shop_items(name, description, price, stock, publisher_id)
		VALUES(?,?,?,?,?)`, req.Name, req.Description, req.Price, req.Stock, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "shop_create", req.Name, "price="+strconv.Itoa(req.Price), clientIP(r))
	OK(w, map[string]any{"id": id, "name": req.Name})
}

// shopUpdate 教师/管理员编辑商品。
func (s *Server) shopUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "商品编号无效")
		return
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM shop_items WHERE id=?`, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists == 0 {
		Fail(w, http.StatusNotFound, "商品不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Price       *int    `json:"price"`
		Stock       *int    `json:"stock"`
		Status      *int    `json:"status"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == nil && req.Price == nil && req.Stock == nil && req.Status == nil && req.Description == nil {
		Fail(w, http.StatusBadRequest, "没有需要更新的字段")
		return
	}
	var name, desc string
	_ = s.db.QueryRow(`SELECT name, description FROM shop_items WHERE id=?`, id).Scan(&name, &desc)
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		desc = *req.Description
	}
	var price, stock, status int
	_ = s.db.QueryRow(`SELECT price, stock, status FROM shop_items WHERE id=?`, id).Scan(&price, &stock, &status)
	if req.Price != nil {
		price = *req.Price
	}
	if req.Stock != nil {
		stock = *req.Stock
	}
	if req.Status != nil {
		status = *req.Status
	}
	if price < 0 || stock < 0 {
		Fail(w, http.StatusBadRequest, "价格与库存不能为负数")
		return
	}
	if _, err := s.db.Exec(`UPDATE shop_items SET name=?, description=?, price=?, stock=?, status=?
		WHERE id=?`, name, desc, price, stock, status, id); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "shop_update", strconv.FormatInt(id, 10), name, clientIP(r))
	OK(w, map[string]any{"id": id})
}

// shopDelete 下架并删除商品。
func (s *Server) shopDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "商品编号无效")
		return
	}
	res, err := s.db.Exec(`DELETE FROM shop_items WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "商品不存在")
		return
	}
	logOp(s.db, claims, "shop_delete", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id, "deleted": true})
}

// shopRedeem 学生兑换商品：校验余额与库存后原子扣减。
func (s *Server) shopRedeem(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		ItemID int64 `json:"item_id"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ItemID <= 0 {
		Fail(w, http.StatusBadRequest, "商品编号无效")
		return
	}

	// 单连接无法开事务（Begin 占住唯一连接，后续查询必然超时），
	// 改为顺序单语句写入：用条件更新保证不超卖、不扣成负分。
	var name, desc string
	var price, status int
	err := s.db.QueryRow(`SELECT name, description, price, status FROM shop_items WHERE id=?`,
		req.ItemID).Scan(&name, &desc, &price, &status)
	if err == sql.ErrNoRows {
		Fail(w, http.StatusNotFound, "商品不存在或已下架")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if status != 1 {
		Fail(w, http.StatusConflict, "商品已下架")
		return
	}

	balance, err := s.userPoints(claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if balance < price {
		Fail(w, http.StatusPaymentRequired, "积分不足，当前 "+strconv.Itoa(balance)+"，需要 "+strconv.Itoa(price))
		return
	}

	// 条件更新扣库存，失败则不发分。
	res, err := s.db.Exec(`UPDATE shop_items SET stock=stock-1 WHERE id=? AND stock>0`, req.ItemID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusConflict, "商品库存不足")
		return
	}
	// 条件更新扣积分，失败则回退库存。
	up, err := s.db.Exec(`UPDATE users SET points=points-? WHERE id=? AND points>=?`,
		price, claims.UserID, price)
	if err != nil {
		_, _ = s.db.Exec(`UPDATE shop_items SET stock=stock+1 WHERE id=?`, req.ItemID)
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := up.RowsAffected(); n == 0 {
		_, _ = s.db.Exec(`UPDATE shop_items SET stock=stock+1 WHERE id=?`, req.ItemID)
		Fail(w, http.StatusPaymentRequired, "积分不足或状态已变化")
		return
	}
	if _, err := s.db.Exec(`INSERT INTO shop_orders(user_id, item_id, item_name, price) VALUES(?,?,?,?)`,
		claims.UserID, req.ItemID, name, price); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := s.db.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, claims.UserID, -price, CategoryShop, "shop", req.ItemID, "兑换："+name); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"item_id": req.ItemID, "item_name": name, "price": price,
		"balance": balance - price, "description": desc})
}

// shopOrders 我的兑换记录。
func (s *Server) shopOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, item_id, item_name, price, created_at FROM shop_orders
		WHERE user_id=? ORDER BY id DESC LIMIT 100`, claims.UserID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type orderRow struct {
		ID        int64  `json:"id"`
		ItemID    int64  `json:"item_id"`
		ItemName  string `json:"item_name"`
		Price     int    `json:"price"`
		CreatedAt string `json:"created_at"`
	}
	list := []orderRow{}
	for rows.Next() {
		var it orderRow
		var created time.Time
		if err := rows.Scan(&it.ID, &it.ItemID, &it.ItemName, &it.Price, (*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		list = append(list, it)
	}
	OK(w, list)
}

// shopAdminList 教师/管理员查看全部商品，含已下架。
func (s *Server) shopAdminList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT i.id, i.name, i.description, i.price, i.stock, i.status,
		COALESCE(u.username, '') FROM shop_items i LEFT JOIN users u ON u.id=i.publisher_id
		ORDER BY i.id DESC LIMIT 500`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []ShopItemResp{}
	for rows.Next() {
		var it ShopItemResp
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &it.Price, &it.Stock,
			&it.Status, &it.Publisher); err != nil {
			continue
		}
		list = append(list, it)
	}
	OK(w, list)
}

package app

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// appVersion 应用版本，可由构建时 -ldflags 注入。
var appVersion = "1.0.0"

// nodeCreate 管理员新增测评节点。
func (s *Server) nodeCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		JudgeType string `json:"judge_type"`
		Status    int    `json:"status"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		Fail(w, http.StatusBadRequest, "节点名称不能为空")
		return
	}
	if req.JudgeType == "" {
		req.JudgeType = "cpp"
	}
	res, err := s.db.Exec(`INSERT INTO judge_nodes(name, judge_type, status) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET judge_type=excluded.judge_type, status=excluded.status,
		last_seen=CURRENT_TIMESTAMP`, name, req.JudgeType, req.Status)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	logOp(s.db, claims, "node_create", name, "type="+req.JudgeType, clientIP(r))
	OK(w, map[string]any{"id": id, "name": name})
}

// nodeDelete 管理员删除测评节点。
func (s *Server) nodeDelete(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "节点编号无效")
		return
	}
	res, err := s.db.Exec(`DELETE FROM judge_nodes WHERE id=?`, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(w, http.StatusNotFound, "节点不存在")
		return
	}
	logOp(s.db, claims, "node_delete", strconv.FormatInt(id, 10), "", clientIP(r))
	OK(w, map[string]any{"id": id, "deleted": true})
}

// serviceStart 进程启动时间，用于服务状态接口。
var serviceStart = time.Now()

// runtimeCPU 保留 runtime 引用，便于健康检查侧排查进程资源状况。
var runtimeCPU = runtime.NumCPU()

// NodeInfo 测评节点，同时用于服务状态概览与节点管理列表。
type NodeInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	JudgeType   string `json:"judge_type"`
	TotalCount  int    `json:"total_count"`
	AcceptCount int    `json:"accept_count"`
	Running     int    `json:"running"`
	CreatedAt   string `json:"created_at"`
	LastSeen    string `json:"last_seen"`
	AgeSeconds  int64  `json:"age_seconds"`
}

// StatusResp 服务状态响应，公开接口。
type StatusResp struct {
	Status  string     `json:"status"`
	Version string     `json:"version"`
	Uptime  int64      `json:"uptime_seconds"`
	Started string     `json:"started_at"`
	Queue   QueueStats `json:"queue"`
	Node    NodeInfo   `json:"node"`
}

// serviceStatus 返回服务与测评队列的健康概况，无需登录。
func (s *Server) serviceStatus(w http.ResponseWriter, r *http.Request) {
	node := NodeInfo{Name: "builtin", JudgeType: "cpp"}
	var created, seen time.Time
	err := s.db.QueryRow(`SELECT id, name, status, judge_type, total_count, accept_count, running,
		created_at, last_seen FROM judge_nodes ORDER BY id LIMIT 1`).
		Scan(&node.ID, &node.Name, &node.Status, &node.JudgeType, &node.TotalCount, &node.AcceptCount,
			&node.Running, (*time.Time)(&created), (*time.Time)(&seen))
	if err == nil {
		node.CreatedAt, node.LastSeen = created.String(), seen.String()
	}

	resp := StatusResp{
		Status:  "running",
		Version: appVersion,
		Uptime:  int64(time.Since(serviceStart).Seconds()),
		Started: serviceStart.Format(time.RFC3339),
		Queue:   s.Queue.Stats(),
		Node:    node,
	}
	// 队列接近满载或节点异常时给出可读提示，前端可据此变色。
	if resp.Queue.Capacity > 0 && resp.Queue.Pending*10 >= resp.Queue.Capacity*9 {
		resp.Status = "busy"
	}
	if resp.Node.Status != 0 {
		resp.Status = "degraded"
	}
	OK(w, resp)
}

// RecentJudgeItem 队列监控中的最近判题记录。
type RecentJudgeItem struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	ProblemName string `json:"problem_name"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	TimeUsed    int    `json:"time_used"`
	MemUsed     int    `json:"mem_used"`
	CreatedAt   string `json:"created_at"`
}

// QueueMonitor 队列监控响应。
type QueueMonitor struct {
	Stats              QueueStats        `json:"stats"`
	PendingSubmissions int               `json:"pending_submissions"`
	Recent             []RecentJudgeItem `json:"recent"`
}

// queueStats 队列监控详情，供管理后台轮询。
func (s *Server) queueStats(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	_ = claims
	resp := QueueMonitor{Stats: s.Queue.Stats(), Recent: []RecentJudgeItem{}}

	var pending int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE status=?`, StatusPending).Scan(&pending)
	resp.PendingSubmissions = pending

	rows, err := s.db.Query(`SELECT id, username, problem_name, status, time_used, mem_used, created_at
		FROM submissions WHERE status>=1 ORDER BY id DESC LIMIT 20`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var it RecentJudgeItem
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Username, &it.ProblemName, &it.Status, &it.TimeUsed, &it.MemUsed,
			(*time.Time)(&created)); err != nil {
			continue
		}
		it.CreatedAt = created.String()
		if t, ok := StatusText[it.Status]; ok {
			it.StatusText = t
		}
		resp.Recent = append(resp.Recent, it)
	}
	OK(w, resp)
}

// nodeList 列出全部测评节点。
func (s *Server) nodeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, name, status, judge_type, total_count, accept_count, running,
		created_at, last_seen FROM judge_nodes ORDER BY id`)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []NodeInfo{}
	for rows.Next() {
		var n NodeInfo
		var created, seen time.Time
		if err := rows.Scan(&n.ID, &n.Name, &n.Status, &n.JudgeType, &n.TotalCount, &n.AcceptCount,
			&n.Running, (*time.Time)(&created), (*time.Time)(&seen)); err != nil {
			continue
		}
		n.CreatedAt, n.LastSeen = created.String(), seen.String()
		list = append(list, n)
	}
	OK(w, list)
}

// nodeUpdate 更新测评节点状态，用于启用或停用节点。
func (s *Server) nodeUpdate(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "节点编号无效")
		return
	}
	var req struct {
		Status int `json:"status"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	// SQLite 的空更新不反映真实影响行数，因此先确认行存在再改。
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM judge_nodes WHERE id=?`, id).Scan(&exists); err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists == 0 {
		Fail(w, http.StatusNotFound, "节点不存在")
		return
	}
	_, err := s.db.Exec(`UPDATE judge_nodes SET status=?, last_seen=CURRENT_TIMESTAMP WHERE id=?`,
		req.Status, id)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	logOp(s.db, claims, "node_update", strconv.FormatInt(id, 10),
		"status="+strconv.Itoa(req.Status), clientIP(r))
	OK(w, map[string]any{"id": id, "status": req.Status})
}

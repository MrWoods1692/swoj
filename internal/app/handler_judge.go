package app

import (
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 测评机信息：把「机器现在忙不忙」与「机器按什么参数在跑」放到同一处只读端点，
// 运维看队列水位、管理员调参数后都能在同一页面交叉验证。
// 只读展示，不暴露任何密钥；judge_bin 与 cgroup 挂载点是路径，不含凭证。

// JudgeConfigView 对外展示的测评配置。
type JudgeConfigView struct {
	Workers        int      `json:"workers"`
	PoolSize       int      `json:"pool_size"`
	QueueSize      int      `json:"queue_size"`
	QueueCapacity  int      `json:"queue_capacity"`
	UserTimeoutMS  int      `json:"user_timeout_ms"`
	MemLimitMB     int      `json:"mem_limit_mb"`
	MemExtraMB     int      `json:"mem_extra_mb"`
	CGrouPMount    string   `json:"cgroup_mount"`
	JudgeBin       string   `json:"judge_bin"`
	BuiltinJudge   bool     `json:"builtin_judge"`
	Languages      []string `json:"languages"`
	Compiler       string   `json:"compiler"`
	CompilerPath   string   `json:"compiler_path"`
	CompilerVer    string   `json:"compiler_version"`
	RestartPending bool     `json:"restart_pending"`
}

// judgeOverrides 读取 admin_configs 中已覆盖的数值参数。
// admin_configs 只存文本，逐个转换，解析失败的值忽略。
func (s *Server) judgeOverrides() map[string]int {
	out := map[string]int{}
	rows, err := s.db.Query(`SELECT key, value FROM admin_configs WHERE key LIKE 'judge_%'`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		if n, e := strconv.Atoi(v); e == nil {
			out[k] = n
		}
	}
	return out
}

// judgeConfigView 生成展示视图。超时与内存限制取自内存中的当前值
// （保存时已同步更新），队列容量取自队列实际容量，worker 池取自启动配置。
func (s *Server) judgeConfigView() JudgeConfigView {
	j := s.cfg.Judge.Snapshot()
	lim := s.cfg.Judge.Limits()
	ov := s.judgeOverrides()
	view := JudgeConfigView{
		Workers:       j.Workers,
		PoolSize:      j.PoolSize,
		QueueSize:     j.QueueSize,
		QueueCapacity: s.Queue.Stats().Capacity,
		UserTimeoutMS: lim.UserTimeoutMS,
		MemLimitMB:    lim.MemLimitMB,
		MemExtraMB:    lim.MemExtra,
		CGrouPMount:   j.CGrouPMount,
		JudgeBin:      j.JudgeBin,
		BuiltinJudge:  strings.TrimSpace(j.JudgeBin) == "",
		Languages:     []string{"cpp"},
		Compiler:      "g++",
		CompilerPath:  compilerPath,
		CompilerVer:   compilerVersion,
	}
	// 队列容量与 worker 池在启动时固定，写入 admin_configs 后必须重启才生效。
	view.RestartPending = diff(ov, "judge_workers", view.Workers) ||
		diff(ov, "judge_pool", view.PoolSize) ||
		diff(ov, "judge_queue", view.QueueSize)
	return view
}

// diff 判断覆盖表里的值与当前值是否不同。
func diff(ov map[string]int, k string, cur int) bool {
	v, ok := ov[k]
	return ok && v != cur
}

// 编译器探测结果只算一次：exec 外部命令有开销，且一次进程生命周期内不变。
var (
	compilerOnce    sync.Once
	compilerPath    string
	compilerVersion string
)

func probeCompiler() {
	if p, err := exec.LookPath("g++"); err == nil {
		compilerPath = p
	}
	out, err := exec.Command("g++", "--version").CombinedOutput()
	if err != nil {
		return
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		compilerVersion = line[:i]
	} else {
		compilerVersion = line
	}
}

// RuntimeInfo 服务运行时环境，用于判断测评机是否资源受限。
type RuntimeInfo struct {
	GoVersion    string `json:"go_version"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	NumCPU       int    `json:"num_cpu"`
	GOMAXPROCS   int    `json:"gomaxprocs"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAllocMB   int    `json:"mem_alloc_mb"`
	NumGC        int    `json:"num_gc"`
	DataDir      string `json:"data_dir"`
}

// JudgeInfoResp 测评机状态与配置汇总。
type JudgeInfoResp struct {
	Status        string          `json:"status"`
	Version       string          `json:"version"`
	UptimeSeconds int64           `json:"uptime_seconds"`
	StartedAt     string          `json:"started_at"`
	Runtime       RuntimeInfo     `json:"runtime"`
	Judge         JudgeConfigView `json:"judge"`
	Queue         QueueStats      `json:"queue"`
	Nodes         []NodeInfo      `json:"nodes"`
	NodesTotal    int             `json:"nodes_total"`
	NodesOnline   int             `json:"nodes_online"`
	NodesBusy     int             `json:"nodes_busy"`
}

// nodeAge 距最后心跳的秒数，前端据此判断节点是否失联。
func nodeAge(seen time.Time) int64 {
	if seen.IsZero() {
		return 0
	}
	return int64(time.Since(seen).Seconds())
}

// nodesAll 读取全部测评节点，供状态页展示。
// 在线节点优先，便于首屏就看到可用资源。
func (s *Server) nodesAll() []NodeInfo {
	rows, err := s.db.Query(`SELECT id, name, status, judge_type, total_count, accept_count, running,
		created_at, last_seen FROM judge_nodes ORDER BY status ASC, id ASC`)
	if err != nil {
		return []NodeInfo{}
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
		n.AgeSeconds = nodeAge(seen)
		list = append(list, n)
	}
	return list
}

// judgeInfo 公开端点：返回测评机状态、运行环境与配置参数。
func (s *Server) judgeInfo(w http.ResponseWriter, r *http.Request) {
	compilerOnce.Do(probeCompiler)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	nodes := s.nodesAll()
	online, busy := 0, 0
	for _, n := range nodes {
		if n.Status == 0 {
			online++
		} else if n.Status == 2 {
			busy++
		}
	}

	state := "running"
	q := s.Queue.Stats()
	if q.Capacity > 0 && q.Pending*10 >= q.Capacity*9 {
		state = "busy"
	}
	if online == 0 {
		state = "offline"
	}

	OK(w, JudgeInfoResp{
		Status:        state,
		Version:       appVersion,
		UptimeSeconds: int64(time.Since(serviceStart).Seconds()),
		StartedAt:     serviceStart.Format(time.RFC3339),
		Runtime: RuntimeInfo{
			GoVersion:    runtime.Version(),
			GOOS:         runtime.GOOS,
			GOARCH:       runtime.GOARCH,
			NumCPU:       runtime.NumCPU(),
			GOMAXPROCS:   runtime.GOMAXPROCS(0),
			NumGoroutine: runtime.NumGoroutine(),
			MemAllocMB:   int(m.Alloc / 1024 / 1024),
			NumGC:        int(m.NumGC),
			DataDir:      s.cfg.DataDir,
		},
		Judge:       s.judgeConfigView(),
		Queue:       q,
		Nodes:       nodes,
		NodesTotal:  len(nodes),
		NodesOnline: online,
		NodesBusy:   busy,
	})
}

// judgeConfigSet 管理员调整测评参数。
// 超时与内存限制写入后立即生效（Judge 每次判题实时读取）；
// 队列容量与 worker 池在启动时固定，只能持久化待重启生效，
// 路径类参数（judge_bin、cgroup_mount）必须在环境变量里改，运行中拒绝写入，
// 避免把执行器指到危险目录。
func (s *Server) judgeConfigSet(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireAdminClaims(w, r)
	if !ok {
		return
	}
	var req struct {
		Workers       *int `json:"workers"`
		PoolSize      *int `json:"pool_size"`
		QueueSize     *int `json:"queue_size"`
		UserTimeoutMS *int `json:"user_timeout_ms"`
		MemLimitMB    *int `json:"mem_limit_mb"`
		MemExtraMB    *int `json:"mem_extra_mb"`
	}
	if err := decode(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, err.Error())
		return
	}

	// 区间校验：越界直接拒绝，避免把队列容量写成 0 或内存限制写成负数导致判题全线失败。
	clamp := func(v *int, lo, hi int, label string) error {
		if v == nil {
			return nil
		}
		if *v < lo || *v > hi {
			return &ErrText{msg: "参数 " + label + " 超出允许范围"}
		}
		return nil
	}
	for _, chk := range []struct {
		v    *int
		lo   int
		hi   int
		name string
	}{
		{req.Workers, 1, 64, "workers"},
		{req.PoolSize, 1, 256, "pool_size"},
		{req.QueueSize, 1, 100000, "queue_size"},
		{req.UserTimeoutMS, 100, 600000, "user_timeout_ms"},
		{req.MemLimitMB, 16, 65536, "mem_limit_mb"},
		{req.MemExtraMB, 0, 4096, "mem_extra_mb"},
	} {
		if err := clamp(chk.v, chk.lo, chk.hi, chk.name); err != nil {
			Fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// 持久化到 admin_configs。nil 表示本次未提交该字段，按当前生效值回填，
	// 保证 admin_configs 始终是完整快照。
	lim := s.cfg.Judge.Limits()
	seed := s.cfg.Judge.Snapshot()
	put := map[string]string{
		"judge_workers":    intOr(req.Workers, seed.Workers),
		"judge_pool":       intOr(req.PoolSize, seed.PoolSize),
		"judge_queue":      intOr(req.QueueSize, seed.QueueSize),
		"judge_timeout_ms": intOr(req.UserTimeoutMS, lim.UserTimeoutMS),
		"judge_mem_limit":  intOr(req.MemLimitMB, lim.MemLimitMB),
		"judge_mem_extra":  intOr(req.MemExtraMB, lim.MemExtra),
	}
	for k, v := range put {
		_, err := s.db.Exec(`INSERT INTO admin_configs(key,value) VALUES(?,?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v)
		if err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 只把可热生效的参数写回内存；队列容量与 worker 池保持启动值不变。
	s.cfg.Judge.SetLimits(
		time.Duration(ovInt(put["judge_timeout_ms"], lim.UserTimeoutMS))*time.Millisecond,
		ovInt(put["judge_mem_limit"], lim.MemLimitMB),
		ovInt(put["judge_mem_extra"], lim.MemExtra),
	)

	logOp(s.db, claims, "judge_config_set", "judge",
		"params="+strconv.Itoa(len(put)), clientIP(r))
	OK(w, map[string]any{"applied": true, "view": s.judgeConfigView()})
}

// intOr 指针为 nil 时取默认值，便于把可选字段写成确定字符串。
func intOr(v *int, dflt int) string {
	if v != nil {
		return strconv.Itoa(*v)
	}
	return strconv.Itoa(dflt)
}

// ovInt 把字符串转回整数，转换失败回落默认值。
func ovInt(v string, dflt int) int {
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return dflt
}

// judgeConfigGet 管理员读取当前生效参数与覆盖来源。
func (s *Server) judgeConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminClaims(w, r); !ok {
		return
	}
	OK(w, map[string]any{
		"judge":            s.judgeConfigView(),
		"overridden":       s.judgeOverrides(),
		"restart_required": []string{"judge_bin", "cgroup_mount"},
	})
}

package app

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config 汇总全部运行配置。字段均可通过环境变量覆盖，未设置的取生产默认值。
// Judge 的资源限制会由管理员在运行中调整，读写经过 JudgeConfig 自己的锁。
type Config struct {
	ListenAddr string
	DataDir    string
	StaticDir  string
	HostURL    string

	JWTSecret string
	JWTLife   time.Duration

	Judge  JudgeConfig
	AI     AIConfig
	Points *PointsConfig
	// Levels 等级档位表，未配置时用 DefaultLevelTiers()。
	Levels []LevelTier
	// Campux OAuth 登录。密钥可由管理员控制台写入 admin_configs 覆盖，无需发版即可更换。
	OAuth OAuthConfig
}

// JudgeSnapshot 与 JudgeConfig 同构但不含锁，供并发安全的读取拷贝。
type JudgeSnapshot struct {
	Workers      int
	PoolSize     int
	QueueSize    int
	UserTimeout  time.Duration
	UserMemLimit int
	MemExtra     int
	CGrouPMount  string
	JudgeBin     string
}

// LimitsView 当前生效的资源限制快照。
type LimitsView struct {
	UserTimeoutMS int
	MemLimitMB    int
	MemExtra      int
}

// Limits 返回当前生效的资源限制，供每次判题读取。
func (j *JudgeConfig) Limits() LimitsView {
	j.mu.Lock()
	defer j.mu.Unlock()
	return LimitsView{
		UserTimeoutMS: int(j.UserTimeout.Seconds() * 1000),
		MemLimitMB:    j.UserMemLimit,
		MemExtra:      j.MemExtra,
	}
}

// TimeoutAndExtra 返回判题用超时与附加内存开销（毫秒 / MB）。
func (j *JudgeConfig) TimeoutAndExtra() (time.Duration, int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.UserTimeout, j.MemExtra
}

// SetLimits 原子更新资源限制。
func (j *JudgeConfig) SetLimits(timeout time.Duration, memLimit, memExtra int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.UserTimeout = timeout
	j.UserMemLimit = memLimit
	j.MemExtra = memExtra
}

// PointsConfig 积分规则：可通过环境变量覆盖，未配置取默认值。
type PointsConfig struct {
	// 签到：第 1 天给 base，每多连续一天 +bonus，最多到 max。
	SignBase  int
	SignBonus int
	SignMax   int
	// 难度对应单次 AC 积分；未知难度取 ACDefault。
	AC           map[string]int
	ACDefault    int
	OnlineHours  int // 累计在线时长每满 N 小时计一次
	OnlinePoints int
	// 比赛名次 -> 积分；未在表中的名次取 ContestDefault。
	Contest        map[string]int
	ContestDefault int
}

// DefaultPointsConfig 给出默认积分规则。
func DefaultPointsConfig() *PointsConfig {
	return &PointsConfig{
		SignBase: 1, SignBonus: 1, SignMax: 5,
		AC:          map[string]int{"Easy": 5, "Medium": 10, "Hard": 20, "Legend": 40},
		ACDefault:   10,
		OnlineHours: 1, OnlinePoints: 1,
		Contest:        map[string]int{"1": 50, "2": 30, "3": 20},
		ContestDefault: 10,
	}
}

// OAuthConfig Campux 校园墙 OAuth2 接入参数。
// BaseURL 对应 Campux 站点根地址，三个端点由它推导；
// Secret 只放服务端，不得下发前端。
type OAuthConfig struct {
	BaseURL     string
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	ClientID    string
	Secret      string
	Scope       string
	HostURL     string
	Callback    string
	Login       string
	// Mock 开启后允许回调凭 mockname/mockqq 直接登录，仅用于本地联调与测试。
	Mock bool
}

// OAuthEnabled 判断 OAuth 登录是否已配置完整并可用。
func (c *OAuthConfig) OAuthEnabled() bool {
	return c != nil && c.ClientID != "" && c.Secret != ""
}

// OAuthCallbackURL 拼出完整回调地址，作为 Campux 侧需要登记的地址。
func (c *OAuthConfig) OAuthCallbackURL() string {
	cb := c.Callback
	if cb != "" && cb[0] != '/' {
		cb = "/" + cb
	}
	return strings.TrimRight(c.HostURL, "/") + cb
}

// JudgeConfig 控制测评服务：进程池规模、单用例资源限制与本地 judge 二进制路径。
// mu 保护会被管理员在运行中修改的数值字段，判题路径与保存接口都要经过它。
type JudgeConfig struct {
	mu           sync.Mutex
	Workers      int
	PoolSize     int
	QueueSize    int
	UserTimeout  time.Duration
	UserMemLimit int // MB
	MemExtra     int // MB，编译/栈/库开销，计入 cgroup memory.max
	CGrouPMount  string
	JudgeBin     string // 为空则使用内置执行器
}

// Snapshot 返回当前配置的安全拷贝，供只读视图使用。
func (j *JudgeConfig) Snapshot() JudgeSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JudgeSnapshot{
		Workers:      j.Workers,
		PoolSize:     j.PoolSize,
		QueueSize:    j.QueueSize,
		UserTimeout:  j.UserTimeout,
		UserMemLimit: j.UserMemLimit,
		MemExtra:     j.MemExtra,
		CGrouPMount:  j.CGrouPMount,
		JudgeBin:     j.JudgeBin,
	}
}

// AIConfig 配置 AI 问答/解析代理。APIKey 为空时返回「未配置」提示而非报错。
type AIConfig struct {
	Enabled     bool
	APIURL      string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float32
}

// LoadConfig 从环境变量构建配置。
func LoadConfig() *Config {
	cfg := &Config{
		ListenAddr: envStr("SWOJ_LISTEN", ":8080"),
		DataDir:    envStr("SWOJ_DATA_DIR", "data"),
		StaticDir:  envStr("SWOJ_STATIC", "web/dist"),
		HostURL:    envStr("SWOJ_HOST", "http://localhost:8080"),
		JWTSecret:  envStr("SWOJ_SECRET", "swoj-dev-secret-change-me"),
		JWTLife:    envDuration("SWOJ_TOKEN_LIFE", 24*time.Hour),

		Judge: JudgeConfig{
			Workers:      envInt("SWOJ_JUDGE_WORKERS", 2),
			PoolSize:     envInt("SWOJ_JUDGE_POOL", 4),
			QueueSize:    envInt("SWOJ_JUDGE_QUEUE", 100),
			UserTimeout:  envDuration("SWOJ_JUDGE_TIMEOUT", 1*time.Second),
			UserMemLimit: envInt("SWOJ_MEM_LIMIT", 256),
			MemExtra:     64,
			CGrouPMount:  envStr("SWOJ_CGROUP", "/sys/fs/cgroup"),
			JudgeBin:     envStr("SWOJ_JUDGE_BIN", ""),
		},

		AI: AIConfig{
			Enabled:     envBool("SWOJ_AI_ENABLED", false),
			APIURL:      envStr("SWOJ_AI_URL", "https://api.openai.com/v1/chat/completions"),
			APIKey:      envStr("SWOJ_AI_KEY", ""),
			Model:       envStr("SWOJ_AI_MODEL", "gpt-4o-mini"),
			MaxTokens:   envInt("SWOJ_AI_MAX_TOKENS", 1024),
			Temperature: float32(envFloat("SWOJ_AI_TEMP", 0.4)),
		},
		Points: loadPointsConfig(),
		OAuth:  loadOAuthConfig(),
	}
	return cfg
}

// 回调路径固定为 /auth/campux/callback；Campux OAuth 应用里登记的地址必须与此完全一致，
// 协议、域名、路径、末尾斜杠任一不同都会导致 redirect_uri mismatch 或未注册。
const oauthCallbackPath = "/auth/campux/callback"

// loadOAuthConfig 从环境变量读取 Campux OAuth 接入参数，未设置时用默认值。
// 密钥建议只放服务端环境变量，不要提交进代码仓库。
func loadOAuthConfig() OAuthConfig {
	base := envStr("SWOJ_OAUTH_BASE", "http://kg.campux.top")
	base = strings.TrimRight(base, "/")
	c := OAuthConfig{
		BaseURL:     base,
		AuthURL:     base + "/oauth/authorize",
		TokenURL:    base + "/oauth/token",
		UserInfoURL: base + "/oauth/userinfo",
		ClientID:    envStr("SWOJ_OAUTH_CLIENT_ID", "4ROQNWLOP5zRkhQe"),
		Secret:      envStr("SWOJ_OAUTH_SECRET", ""),
		Scope:       envStr("SWOJ_OAUTH_SCOPE", "profile tenant"),
		HostURL:     envStr("SWOJ_HOST", "http://localhost:8080"),
		Callback:    oauthCallbackPath,
		Login:       "/login",
		Mock:        envBool("SWOJ_OAUTH_MOCK", false),
	}
	return c
}

func envStr(k, d string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v, ok := os.LookupEnv(k); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

func envBool(k string, d bool) bool {
	if v, ok := os.LookupEnv(k); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return d
}

func envFloat(k string, d float64) float64 {
	if v, ok := os.LookupEnv(k); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return d
}

// loadPointsConfig 从环境变量读取积分规则，未设置时用默认值。
// 难度分与比赛分用逗号分隔的「键:值」对配置，便于不改代码调规则。
func loadPointsConfig() *PointsConfig {
	p := DefaultPointsConfig()
	p.SignBase = envInt("SWOJ_SIGN_BASE", p.SignBase)
	p.SignBonus = envInt("SWOJ_SIGN_BONUS", p.SignBonus)
	p.SignMax = envInt("SWOJ_SIGN_MAX", p.SignMax)
	p.ACDefault = envInt("SWOJ_AC_DEFAULT", p.ACDefault)
	p.OnlineHours = envInt("SWOJ_ONLINE_HOURS", p.OnlineHours)
	p.OnlinePoints = envInt("SWOJ_ONLINE_POINTS", p.OnlinePoints)
	p.ContestDefault = envInt("SWOJ_CONTEST_DEFAULT", p.ContestDefault)
	if m := parseKVMap(envStr("SWOJ_AC_POINTS", "")); m != nil {
		p.AC = m
	}
	if m := parseKVMap(envStr("SWOJ_CONTEST_POINTS", "")); m != nil {
		p.Contest = m
	}
	return p
}

// parseKVMap 解析 "Easy:5,Medium:10" 形式的键值对；解析失败返回 nil。
func parseKVMap(s string) map[string]int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	m := map[string]int{}
	for _, kv := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(kv), ":", 2)
		if len(parts) != 2 {
			return nil
		}
		if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			m[strings.TrimSpace(parts[0])] = n
		}
	}
	return m
}

func envDuration(k string, d time.Duration) time.Duration {
	if v, ok := os.LookupEnv(k); ok {
		if sec, err := strconv.Atoi(v); err == nil {
			return time.Duration(sec) * time.Second
		}
	}
	return d
}

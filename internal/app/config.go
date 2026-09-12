package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇总全部运行配置。字段均可通过环境变量覆盖，未设置的取生产默认值。
type Config struct {
	ListenAddr string
	DataDir    string
	StaticDir  string
	HostURL    string

	JWTSecret string
	JWTLife   time.Duration

	AdminUser string
	AdminPass string

	Judge  JudgeConfig
	AI     AIConfig
	Points *PointsConfig
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

// JudgeConfig 控制测评服务：进程池规模、单用例资源限制与本地 judge 二进制路径。
type JudgeConfig struct {
	Workers      int
	PoolSize     int
	QueueSize    int
	UserTimeout  time.Duration
	UserMemLimit int // MB
	MemExtra     int // MB，编译/栈/库开销，计入 cgroup memory.max
	CGrouPMount  string
	JudgeBin     string // 为空则使用内置执行器
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
		AdminUser:  envStr("SWOJ_ADMIN_USER", "admin"),
		AdminPass:  envStr("SWOJ_ADMIN_PASS", "admin123"),

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
	}
	return cfg
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

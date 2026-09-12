package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 测评状态码，与主流 OJ 语义对齐。
const (
	StatusPending    = 0
	StatusAccepted   = 1
	StatusWrongAns   = 2
	StatusCompileErr = 3
	StatusTLE        = 4
	StatusMLE        = 5
	StatusRE         = 6
	StatusPE         = 7
	StatusSE         = 8
)

// StatusText 状态中文描述。
var StatusText = map[int]string{
	StatusAccepted:   "通过",
	StatusWrongAns:   "答案错误",
	StatusCompileErr: "编译错误",
	StatusTLE:        "超时",
	StatusMLE:        "内存超限",
	StatusRE:         "运行时错误",
	StatusPE:         "格式错误",
	StatusSE:         "系统错误",
}

// JudgeResult 单次提交评测结果。
type JudgeResult struct {
	Status     int
	TimeUsed   int
	MemUsed    int
	Message    string
	CompileLog string
}

// Judge 测评器：编译 C++ 源码并逐用例执行，资源限制通过 rlimit 施加。
type Judge struct {
	db       *DB
	cfg      *JudgeConfig
	points   *PointsConfig // 难度 -> 积分映射，AC 时入账
	pool     chan struct{}
	baseDir  string
	wrapOnce sync.Once
	wrapPath string
	wrapErr  error
}

// NewJudge 创建测评器。
func NewJudge(db *DB, cfg *JudgeConfig, points *PointsConfig, baseDir string) *Judge {
	if cfg.UserMemLimit <= 0 {
		cfg.UserMemLimit = 256
	}
	if cfg.UserTimeout <= 0 {
		cfg.UserTimeout = 1 * time.Second
	}
	if points == nil {
		points = DefaultPointsConfig()
	}
	return &Judge{db: db, cfg: cfg, points: points, pool: make(chan struct{}, cfg.PoolSize), baseDir: baseDir}
}

// Run 测评一条提交：编译 → 逐用例执行 → 写回状态。
func (j *Judge) Run(t JudgeTask) error {
	select {
	case j.pool <- struct{}{}:
		defer func() { <-j.pool }()
	default:
		return ErrQueueFull
	}

	subDir := filepath.Join(j.baseDir, fmt.Sprintf("%d", t.SubID))
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(subDir) }()

	srcPath := filepath.Join(subDir, "main.cpp")
	if err := os.WriteFile(srcPath, []byte(t.Code), 0o644); err != nil {
		return fmt.Errorf("write source: %w", err)
	}

	p, err := j.loadProblem(t.ProblemID)
	if err != nil {
		return err
	}

	res, err := j.eval(subDir, srcPath, p)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	return j.writeResult(t.SubID, res, p)
}

// eval 执行编译与逐用例判定。
func (j *Judge) eval(subDir, srcPath string, p Problem) (JudgeResult, error) {
	binPath := filepath.Join(subDir, "main")

	compileLog, err := j.compile(srcPath, binPath)
	if err != nil || strings.TrimSpace(compileLog) != "" {
		return JudgeResult{
			Status: StatusCompileErr, Message: "编译错误",
			CompileLog: strings.TrimSpace(compileLog),
		}, nil
	}

	cases, err := j.loadCases(p.ID)
	if err != nil {
		return JudgeResult{}, err
	}

	tl := time.Duration(p.TimeLimit) * time.Millisecond
	if tl <= 0 {
		tl = j.cfg.UserTimeout
	}
	memoryKB := p.MemLimit*1024 + j.cfg.MemExtra*1024
	if memoryKB <= 0 {
		memoryKB = 256 * 1024
	}

	inPath := filepath.Join(subDir, "input.txt")
	outPath := filepath.Join(subDir, "output.txt")

	var lastTime, lastMem int
	for _, c := range cases {
		if err := os.WriteFile(inPath, []byte(c.Input), 0o644); err != nil {
			return JudgeResult{}, err
		}
		_ = os.Remove(outPath)

		fsizeKB := p.FileLimit
		if fsizeKB <= 0 {
			fsizeKB = 64
		}
		r, err := j.execute(binPath, outPath, subDir, tl, memoryKB, fsizeKB)
		if err != nil {
			return JudgeResult{Status: StatusRE, Message: "运行时错误"}, nil
		}
		lastTime, lastMem = r.TimeUsed, r.MemUsed

		switch {
		case r.TLE:
			return JudgeResult{Status: StatusTLE, Message: "超时", TimeUsed: r.TimeUsed, MemUsed: r.MemUsed}, nil
		case r.MLE:
			return JudgeResult{Status: StatusMLE, Message: "内存超限", TimeUsed: r.TimeUsed, MemUsed: r.MemUsed}, nil
		case r.ExitCode != 0:
			return JudgeResult{Status: StatusRE, Message: "运行时错误", TimeUsed: r.TimeUsed, MemUsed: r.MemUsed}, nil
		}

		got, err := os.ReadFile(outPath)
		if err != nil {
			return JudgeResult{Status: StatusSE, Message: "系统错误"}, nil
		}
		if strings.TrimSpace(string(got)) != strings.TrimSpace(c.Output) {
			return JudgeResult{Status: StatusWrongAns, Message: "答案错误", TimeUsed: r.TimeUsed, MemUsed: r.MemUsed}, nil
		}
	}

	return JudgeResult{Status: StatusAccepted, Message: "通过", TimeUsed: lastTime, MemUsed: lastMem}, nil
}

// compile 编译 C++ 源码。
func (j *Judge) compile(src, bin string) (string, error) {
	cmd := exec.Command("g++", "-O2", "-std=c++17", "-w", "-o", bin, src)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// execResult 单次用例执行统计。
type execResult struct {
	ExitCode int
	TimeUsed int
	MemUsed  int
	TLE      bool
	MLE      bool
}

// execute 通过 C 包装器运行用户程序：rlimit 施加 CPU/内存/文件大小上限，
// Setpgid 保证超时时能整组回收孙进程，stderr 携带「CPU耗时 峰值内存」统计。
func (j *Judge) execute(bin, outPath, workDir string, tl time.Duration, memKB, fsizeKB int) (execResult, error) {
	wrap, err := j.compileWrapper()
	if err != nil {
		return execResult{}, err
	}
	ms := int(tl / time.Millisecond)

	cmd := exec.Command(wrap, fmt.Sprintf("%d", ms), fmt.Sprintf("%d", memKB),
		fmt.Sprintf("%d", fsizeKB), bin)
	cmd.Dir = workDir
	inFile, err := os.Open(filepath.Join(workDir, "input.txt"))
	if err != nil {
		return execResult{}, err
	}
	cmd.Stdin = inFile
	cmd.Env = append(os.Environ(),
		"SWOJ_OUT="+outPath,
		"SWOJ_IN="+filepath.Join(workDir, "input.txt"))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 包装器把「CPU耗时 峰值内存」写到 stderr，stdout 留给用户程序经 SWOJ_OUT 落盘。
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	tlEnd := time.AfterFunc(tl+2*time.Second, func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	err = cmd.Run()
	tlEnd.Stop()
	_ = inFile.Close()

	res := execResult{ExitCode: 0}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ProcessState.ExitCode()
		} else {
			return execResult{}, err
		}
	}
	j.parseStats(errBuf.Bytes(), &res)
	// 152 = 128 + SIGXCPU(24)：RLIMIT_CPU 耗尽，语义上就是 CPU 超时，
	// 必须判为 TLE 而不是笼统的运行时错误。
	// 152 = 128 + SIGXCPU(24)：RLIMIT_CPU 耗尽，语义上就是 CPU 超时。
	// 137/143 分别是 Go 侧看门狗发出的 SIGKILL/SIGTERM 兜底回收。
	if res.ExitCode == 128+24 || res.ExitCode == 137 || res.ExitCode == 143 {
		res.TLE = true
	}
	return res, nil
}

// parseStats 从包装器 stderr 解析「CPU耗时 峰值内存」。
func (j *Judge) parseStats(out []byte, res *execResult) {
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		t, _ := strconv.Atoi(fields[0])
		m, _ := strconv.Atoi(fields[1])
		res.TimeUsed, res.MemUsed = t, m
	}
}

// wrapperSource 是编译期生成的 C 包装器源码。它在 exec 用户程序前设置
// RLIMIT_CPU / RLIMIT_AS / RLIMIT_FSIZE，并收集子进程 CPU 与峰值内存。
// 原因：本环境无 cgroup 写权限，进程级 rlimit 是唯一可靠的资源隔离手段。
const wrapperSource = `
#define _GNU_SOURCE
#include <sys/resource.h>
#include <sys/time.h>
#include <sys/wait.h>
#include <time.h>
#include <sys/types.h>
#include <unistd.h>
#include <signal.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv) {
    if (argc < 5) return 97;
    long cpu_ms = atol(argv[1]);
    long mem_kb = atol(argv[2]);
    long fsize_kb = atol(argv[3]);
    char *bin = argv[4];

    int outfd = -1;
    char *outp = getenv("SWOJ_OUT");
    if (outp) outfd = open(outp, O_WRONLY|O_CREAT|O_TRUNC, 0644);
    if (outfd < 0) outfd = 1;
    // 只接管 stdout：stderr 必须留给包装器回传「CPU 峰值内存」统计，
    // 否则 Go 侧的 cmd.Stderr 永远收不到统计行。
    dup2(outfd, 1);

    struct rlimit cpu = { cpu_ms/1000, cpu_ms/1000 };
    struct rlimit as  = { (rlim_t)mem_kb*1024, (rlim_t)mem_kb*1024 };
    struct rlimit fs  = { (rlim_t)fsize_kb*1024, (rlim_t)fsize_kb*1024 };
    setrlimit(RLIMIT_CPU, &cpu);
    setrlimit(RLIMIT_AS, &as);
    setrlimit(RLIMIT_FSIZE, &fs);
    setrlimit(RLIMIT_NOFILE, &(struct rlimit){128,128});

    pid_t pid = fork();
    if (pid == 0) {
        setsid();
        char *inp = getenv("SWOJ_IN");
        if (inp) {
            int infd = open(inp, O_RDONLY);
            if (infd >= 0) { dup2(infd, 0); close(infd); }
        }
        for (int fd = 3; fd < 64; fd++) close(fd);
        execv(bin, &argv[4]);
        _exit(127);
    }

    struct timespec t0, t1;
    clock_gettime(CLOCK_MONOTONIC, &t0);
    int status;
    waitpid(pid, &status, 0);
    clock_gettime(CLOCK_MONOTONIC, &t1);
    if (outfd >= 0) close(outfd);

    long cpu_used = (t1.tv_sec - t0.tv_sec) * 1000 +
                    (t1.tv_nsec - t0.tv_nsec) / 1000000;
    struct rusage ru;
    getrusage(RUSAGE_CHILDREN, &ru);
    long mem_used = ru.ru_maxrss;

    // 统计必须走真实 stderr：stdout 已被 dup2 到输出文件，
    // 而 Go 侧的 cmd.Stderr 是包装器自身未重定向的错误流。
    FILE *errf = fdopen(2, "w");
    if (errf) {
        fprintf(errf, "%ld %ld", cpu_used, mem_used);
        fclose(errf);
    } else {
        dprintf(2, "%ld %ld", cpu_used, mem_used);
    }

    if (WIFEXITED(status)) return WEXITSTATUS(status);
    if (WIFSIGNALED(status)) return 128 + WTERMSIG(status);
    return 97;
}
`

// compileWrapper 编译 C 包装器。用 sync.Once 保证整个进程只编译一次：
// 多个 worker 并发判题时若各自编译，会同时写同一文件导致包装器损坏。
func (j *Judge) compileWrapper() (string, error) {
	j.wrapOnce.Do(func() {
		wrap := filepath.Join(j.baseDir, "wrapper.c")
		bin := filepath.Join(j.baseDir, "wrapper")
		if err := os.WriteFile(wrap, []byte(wrapperSource), 0o644); err != nil {
			j.wrapErr = fmt.Errorf("write wrapper: %w", err)
			return
		}
		cmd := exec.Command("gcc", "-O2", "-std=gnu99", "-o", bin, wrap)
		if out, err := cmd.CombinedOutput(); err != nil {
			j.wrapErr = fmt.Errorf("build wrapper: %v %s", err, out)
			return
		}
		j.wrapPath = bin
	})
	return j.wrapPath, j.wrapErr
}

// loadProblem 读取题目及其资源限制。
func (j *Judge) loadProblem(id int64) (Problem, error) {
	p := Problem{}
	row := j.db.QueryRow(`SELECT id, name, difficulty, time_limit, mem_limit, file_limit, stack_limit FROM problems WHERE id=?`, id)
	err := row.Scan(&p.ID, &p.Name, &p.Difficulty, &p.TimeLimit, &p.MemLimit, &p.FileLimit, &p.StackLimit)
	if err != nil {
		return p, fmt.Errorf("problem not found: %w", err)
	}
	return p, nil
}

// awardACPts 为一次 AC 发放难度积分。
// 仅当用户此前未通过该题时才发放，防止重复提交刷分。
func (j *Judge) awardACPts(userID, problemID int64, difficulty string) {
	if j.points == nil {
		return
	}
	var n int
	if err := j.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=? AND problem_id=? AND status=?`,
		userID, problemID, StatusAccepted).Scan(&n); err != nil {
		return
	}
	if n > 1 {
		return
	}
	gain := j.points.acPoints(difficulty)
	if gain <= 0 {
		return
	}
	tx, err := j.db.Begin()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE users SET points = points + ? WHERE id=?`, gain, userID); err != nil {
		return
	}
	if _, err := tx.Exec(`INSERT INTO points_log(user_id, delta, category, ref_type, ref_id, remark)
		VALUES(?,?,?,?,?,?)`, userID, gain, CategoryAC, "problem", problemID,
		fmt.Sprintf("通过题目[%s] +%d", difficulty, gain)); err != nil {
		return
	}
	_ = tx.Commit()
}

// loadCases 按顺序读取全部测试用例。
func (j *Judge) loadCases(problemID int64) ([]CaseIO, error) {
	rows, err := j.db.Query(`SELECT id, problem_id, index_no, input, output FROM cases WHERE problem_id=? ORDER BY index_no`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cases []CaseIO
	for rows.Next() {
		var c CaseIO
		if err := rows.Scan(&c.ID, &c.ProblemID, &c.Index, &c.Input, &c.Output); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no test cases for problem %d", problemID)
	}
	return cases, nil
}

// writeResult 把评测结果写回提交记录并同步题目统计与错题本。
func (j *Judge) writeResult(subID int64, r JudgeResult, p Problem) error {
	var sub struct {
		UserID int64
		Name   string
	}
	_ = j.db.QueryRow(`SELECT user_id, username FROM submissions WHERE id=?`, subID).Scan(&sub.UserID, &sub.Name)

	_, err := j.db.Exec(`UPDATE submissions SET status=?, msg=?, time_used=?, mem_used=?, error=? WHERE id=?`,
		r.Status, r.Message, r.TimeUsed, r.MemUsed, r.CompileLog, subID)
	if err != nil {
		return err
	}

	_, _ = j.db.Exec(`UPDATE problems SET submit = submit + 1 WHERE id=?`, p.ID)

	if r.Status == StatusAccepted {
		_, _ = j.db.Exec(`UPDATE problems SET accept = accept + 1 WHERE id=?`, p.ID)
		_, _ = j.db.Exec(`UPDATE users SET problem_count = problem_count + 1 WHERE id=?`, sub.UserID)
		// 难度积分：仅首次通过该题计入，避免重复刷分。
		j.awardACPts(sub.UserID, p.ID, p.Difficulty)
	}
	if r.Status != StatusAccepted {
		_, _ = j.db.Exec(`INSERT INTO wrong_questions(user_id, problem_id, problem_name, times, last_try_at)
			VALUES(?,?,?,1,datetime('now'))
			ON CONFLICT(user_id, problem_id) DO UPDATE SET times = times + 1, last_try_at = datetime('now')`,
			sub.UserID, p.ID, p.Name)
	}
	// 成就检测：任何提交（AC 或 WA）都可能触发首题/首 WA/AC 计数类成就。
	_, _ = dbRefreshAchievements(j.db, sub.UserID)
	return nil
}

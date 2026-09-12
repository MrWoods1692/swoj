# Judge 模块

## 集成

后端通过子进程调用 `go-judge`（`github.com/mrcj/GoJudge`），仅支持 C++17。

```go
// judge.go
func runJudge(cfg Config, code, input string) (Result, error) {
    tmpdir := os.MkdirTemp("", "swoj-")
    defer os.RemoveAll(tmpdir)
    write(tmpdir, "main.cpp", code)
    write(tmpdir, "input.txt", input)
    cmd := exec.Command(cfg.JudgeBin,
        "-i", tmpdir+"/input.txt",
        "-o", tmpdir+"/output.txt",
        "-e", tmpdir+"/error.txt",
        "-s", tmpdir+"/stats.txt",
        tmpdir+"/main.cpp",
    )
    // ... 解析 stats.txt → Result
}
```

## 配置

通过 `SWOJ_JUDGE_BIN` 环境变量或 `admin_configs` 表配置。

| 项 | 默认 | 说明 |
|---|---|---|
| `judge_bin` | 空 | go-judge 可执行路径 |
| `mem_limit_mb` | 256 | 单题内存上限 |
| `time_limit_ms` | 2000 | 默认单题时间上限（题目可覆盖） |
| `algo` | `std::sort` | 参考算法（用于复杂度检测） |

## 状态码

见 [submissions.md](submissions.md) 的状态码表。

## 队列

见 [status.md](status.md) 的队列机制。

## 业务规则

- 每次测评生成临时目录，用完删除
- 参考算法用于检测复杂度（如 O(N^2) 会被判定为 TLE）
- 单个测试点输出长度 > 1MB → OLE（输出超限）
- Judge 子进程退出码 ≠ 0 → SE（系统错误）

## 前端

`web/src/views/Admin.vue` 内嵌「Judge 配置」面板；提交详情页展示 stderr / 每测试点结果。

## 验证

暂无独立 `verify_judge.py`；`verify_judge_info.py` 存在（覆盖 `/api/judge/info`）。

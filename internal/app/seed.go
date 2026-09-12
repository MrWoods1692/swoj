package app

import "database/sql"

// seed 首次启动时写入内置测评节点与题目。
// 系统不存在任何口令账号：管理员由首位完成校园墙授权的用户自动晋升（见 handler_oauth.go）。
func seed(conn *sql.DB) error {
	// 以测评节点为幂等标记：不再种子任何用户，不能用 users 计数判首启。
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM judge_nodes`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := conn.Exec(`INSERT INTO judge_nodes(name,status,accept_count,judge_type) VALUES('builtin',0,0,'cpp')`); err != nil {
		return err
	}

	return seedProblems(conn)
}

// seedProblem 一条内置题目及其用例。
type seedProblem struct {
	name, diff, content, hint string
	tl, mem                   int
	cases                     [][2]string
}

// seedProblems 写入内置题目、用例与标签，保证首次启动即可完整演示。
func seedProblems(conn *sql.DB) error {
	list := []seedProblem{
		{name: "A+B", diff: "Easy", tl: 1000, mem: 256,
			content: "计算两个整数之和，输出结果。",
			hint:    "注意整数范围可能超出 int。",
			cases:   [][2]string{{"1 2", "3"}, {"100 200", "300"}, {"-5 5", "0"}}},
		{name: "最大公约数", diff: "Easy", tl: 1000, mem: 256,
			content: "求两个正整数的最大公约数。",
			hint:    "辗转相除法。",
			cases:   [][2]string{{"12 18", "6"}, {"100 25", "25"}, {"7 7", "7"}}},
		{name: "斐波那契数列", diff: "Normal", tl: 1000, mem: 256,
			content: "给定 n，输出第 n 个斐波那契数（从 0 开始计数）。",
			hint:    "迭代即可，注意取模。",
			cases:   [][2]string{{"10", "55"}, {"0", "0"}, {"12", "144"}}},
		{name: "快速排序检查", diff: "Normal", tl: 1000, mem: 256,
			content: "给定 n 个整数，按升序输出。",
			hint:    "排序后逐行输出。",
			cases:   [][2]string{{"3\n3 1 2", "1\n2\n3"}, {"1\n7", "7"}, {"4\n4 2 4 1", "1\n2\n4\n4"}}},
	}

	for _, sp := range list {
		res, err := conn.Exec(`INSERT INTO problems(name,difficulty,time_limit,mem_limit,content,hint)
			VALUES(?,?,?,?,?,?)`, sp.name, sp.diff, sp.tl, sp.mem, sp.content, sp.hint)
		if err != nil {
			return err
		}
		pid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for i, c := range sp.cases {
			if _, err := conn.Exec(`INSERT INTO cases(problem_id,index_no,input,output) VALUES(?,?,?,?)`,
				pid, i, c[0], c[1]); err != nil {
				return err
			}
		}
	}
	_, err := conn.Exec(`INSERT OR IGNORE INTO tags(name) VALUES('数学'),('排序'),('字符串'),('图论')`)
	if err != nil {
		return err
	}
	_, err = conn.Exec(`INSERT OR IGNORE INTO training_plans(name,info,creator) VALUES
		('入门算法训练','从基础语法到经典算法的系统训练路径',1),
		('竞赛冲刺计划','面向 NOIP 与 ACM 的高强度训练计划',1)`)
	if err != nil {
		return err
	}
	return nil
}

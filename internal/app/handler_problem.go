package app

import (
	"net/http"
	"strconv"
	"time"
)

// ProblemListResp 题库列表项。
type ProblemListResp struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Difficulty   string   `json:"difficulty"`
	TimeLimit    int      `json:"time_limit"`
	MemLimit     int      `json:"mem_limit"`
	Accept       int      `json:"accept"`
	Submit       int      `json:"submit"`
	AcceptedRate float64  `json:"accept_rate"`
	Tags         []string `json:"tags"`
}

// problemList 题库查询，支持分页、关键词、难度与标签过滤。
func (s *Server) problemList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(q.Get("size"), 10)
	if size < 1 || size > 100 {
		size = 10
	}
	keyword := q.Get("keyword")
	diff := q.Get("difficulty")
	tag := q.Get("tag")

	where := "WHERE 1=1"
	args := []any{}
	if keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	if diff != "" {
		where += " AND difficulty = ?"
		args = append(args, diff)
	}
	if tag != "" {
		where += ` AND id IN (SELECT problem_id FROM problem_tags WHERE tag_id =
(SELECT id FROM tags WHERE name=?))`
		args = append(args, tag)
	}

	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM problems `+where, args...).Scan(&total)

	args = append(args, size, (page-1)*size)
	rows, err := s.db.Query(`SELECT id, name, difficulty, time_limit, mem_limit, accept, submit FROM problems `+
		where+` ORDER BY id ASC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	list := []ProblemListResp{}
	for rows.Next() {
		var p ProblemListResp
		if err := rows.Scan(&p.ID, &p.Name, &p.Difficulty, &p.TimeLimit, &p.MemLimit, &p.Accept, &p.Submit); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if p.Submit > 0 {
			p.AcceptedRate = float64(p.Accept) / float64(p.Submit) * 100
		}
		list = append(list, p)
	}
	OK(w, map[string]any{"list": list, "total": total, "page": page, "size": size})
}

// problemDetail 题目详情，含标签与最近提交。
func (s *Server) problemDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, "题目 ID 无效")
		return
	}
	var p Problem
	err = s.db.QueryRow(`SELECT id, name, difficulty, problem_type, time_limit, mem_limit, file_limit,
		stack_limit, open_data, show_tag, show_code, invisible, accept, submit, content, hint,
		hint_time, created_at FROM problems WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Difficulty, &p.ProblemType, &p.TimeLimit, &p.MemLimit, &p.FileLimit,
			&p.StackLimit, &p.OpenData, &p.ShowTag, &p.ShowCode, &p.Invisible, &p.Accept, &p.Submit,
			&p.Content, &p.Hint, &p.HintTime, (*time.Time)(&p.CreatedAt))
	if err != nil {
		Fail(w, http.StatusNotFound, "题目不存在")
		return
	}
	if p.Submit > 0 {
		p.AcceptedRate = float64(p.Accept) / float64(p.Submit) * 100
	}

	tags := []string{}
	trows, err := s.db.Query(`SELECT t.name FROM tags t JOIN problem_tags pt ON t.id=pt.tag_id
		WHERE pt.problem_id=?`, id)
	if err == nil {
		for trows.Next() {
			var name string
			if trows.Scan(&name) == nil {
				tags = append(tags, name)
			}
		}
		trows.Close()
	}

	OK(w, map[string]any{"problem": p, "tags": tags})
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return d
		}
		n = n*10 + int(c-'0')
	}
	return n
}

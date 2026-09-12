package app

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	RealName     string    `json:"realname"`
	Role         string    `json:"role"`
	School       string    `json:"school"`
	Avatar       string    `json:"avatar"`
	Signature    string    `json:"signature"`
	Website      string    `json:"website"`
	Background   string    `json:"background"`
	QQ           string    `json:"qq"`
	ProblemCount int       `json:"problem_count"`
	RankNo       int       `json:"rank_no"`
	Points       int       `json:"points"`
	Level        int       `json:"level"`
	CanSubmit    bool      `json:"can_submit"`
	CreatedAt    time.Time `json:"created_at"`
	LastLoginAt  time.Time `json:"last_login_at"`
}

type Problem struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Difficulty   string    `json:"difficulty"`
	ProblemType  string    `json:"problem_type"`
	TimeLimit    int       `json:"time_limit"`
	MemLimit     int       `json:"mem_limit"`
	FileLimit    int       `json:"file_limit"`
	StackLimit   int       `json:"stack_limit"`
	OpenData     bool      `json:"open_data"`
	ShowTag      bool      `json:"show_tag"`
	ShowCode     bool      `json:"show_code"`
	Invisible    bool      `json:"invisible"`
	Accept       int       `json:"accept"`
	Submit       int       `json:"submit"`
	AcceptedRate float64   `json:"accept_rate"`
	Content      string    `json:"content"`
	Hint         string    `json:"hint"`
	HintTime     int       `json:"hint_time"`
	CreatedAt    time.Time `json:"created_at"`
}

type CaseIO struct {
	ID        int64
	ProblemID int64
	Index     int
	Input     string
	Output    string
}

type Tag struct {
	ID   int64
	Name string
}

type Submission struct {
	ID           int64
	UserID       int64
	Username     string
	ProblemID    int64
	ProblemName  string
	Code         string
	Mode         string
	ContestID    int64
	AssignmentID int64
	Status       int
	Msg          string
	TimeUsed     int
	MemUsed      int
	Error        string
	CreatedAt    time.Time
}

type Contest struct {
	ID          int64
	Name        string
	Info        string
	Rank        bool
	Visible     bool
	Password    string
	StartTime   time.Time
	EndTime     time.Time
	Creator     int64
	CreatorName string
	Accept      int
	Submit      int
}

type Assignment struct {
	ID          int64
	Name        string
	Info        string
	Visible     bool
	Deadline    time.Time
	Creator     int64
	CreatorName string
}

type TrainingPlan struct {
	ID          int64
	Name        string
	Info        string
	Visible     bool
	Creator     int64
	CreatorName string
	Records     []*TrainingRecord
}

type TrainingRecord struct {
	ID          int64
	PlanID      int64
	ProblemID   int64
	ProblemName string
	Sequence    int
	Status      int
	Accepted    bool
	Times       int
}

type Discussion struct {
	ID        int64
	UserID    int64
	Username  string
	ProblemID int64
	Title     string
	Content   string
	LikeCount int
	CreatedAt time.Time
	Replies   []*Reply
}

type Reply struct {
	ID           int64
	DiscussionID int64
	UserID       int64
	Username     string
	Content      string
	CreatedAt    time.Time
}

type WrongQuestion struct {
	ID          int64
	UserID      int64
	ProblemID   int64
	ProblemName string
	Times       int
	LastTryAt   time.Time
}

type JudgeNode struct {
	ID          int64
	Name        string
	Status      int
	TotalCount  int
	AcceptCount int
	Running     int
	JudgeType   string
	CreatedAt   time.Time
	LastSeen    time.Time
}

type AdminConfig struct {
	Key   string
	Value string
}

type AIQA struct {
	ID        int64
	UserID    int64
	ProblemID int64
	Question  string
	Answer    string
	Source    string
	CreatedAt time.Time
}

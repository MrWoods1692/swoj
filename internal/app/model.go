package app

import "time"

type User struct {
	ID           int64
	Username     string
	Password     string `json:"-"`
	Email        string
	RealName     string
	Role         string
	School       string
	Avatar       string
	Signature    string
	ProblemCount int
	RankNo       int
	CanSubmit    bool
	CreatedAt    time.Time
	LastLoginAt  time.Time
}

type Problem struct {
	ID           int64
	Name         string
	Difficulty   string
	ProblemType  string
	TimeLimit    int
	MemLimit     int
	FileLimit    int
	StackLimit   int
	OpenData     bool
	ShowTag      bool
	ShowCode     bool
	Invisible    bool
	Accept       int
	Submit       int
	AcceptedRate float64
	Content      string
	Hint         string
	HintTime     int
	CreatedAt    time.Time
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

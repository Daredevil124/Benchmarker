package models

type AttackCommand struct {
	SubmissionID        string `json:"submission_id"`
	TargetEndpoint      string `json:"target_endpoint"`
	BotConcurrency      int    `json:"bot_concurrency"`
	TestDurationSeconds int    `json:"test_duration_seconds"`
}
type AttackResult struct {
	BotID      int
	Latency    int64
	StatusCode int
	Error      string
}

type GradingTask struct {
	SubmissionID string `json:"submission_id"`
	Team         string `json:"team"`
	QuestionID   string `json:"question_id"`
	FileName     string `json:"file_name"`
	FileContent  []byte `json:"file_content"`
}

type GradingResult struct {
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"` // AC, WA, TLE, MLE
	LatencyMs    int64  `json:"latency_ms"`
	Output       string `json:"output"`
	ErrorDetail  string `json:"error_detail"`
}

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

package models

type ReviewResult struct {
	Status     string   `json:"status"`     // approved, rejected
	Confidence float64  `json:"confidence"` // 0.0 - 1.0
	Reason     string   `json:"reason"`
	Categories []string `json:"categories"`
}

type ReviewConfig struct {
	Enabled               bool
	TextModel             string
	VisionModel           string
	SampleFrames          int
	FrameReviewMode       string  // "off", "on", "auto"
	ConfidenceThreshold   float64 // High confidence threshold, >= this value AI makes autonomous decision
	ManualReviewThreshold float64 // Gray zone lower bound, < this value and AI says approved -> manual review
	MaxRetries            int
	APIKey                string
	BaseURL               string
	MaxVideoSizeMB        int
	MaxCoverSizeMB        int
	MaxVideoDurationSec   int
	MinVideoDurationSec   int
	EnableAudioReview     bool
	EnableOCRReview       bool
	MaxConcurrentFrames   int
	MaxConcurrentVideos   int
	AgentEnabled          bool
	AgentMaxRounds        int
	AgentTimeoutSec       int
}

func (c ReviewConfig) FrameReviewEnabled() bool {
	return c.FrameReviewMode == "on" || c.FrameReviewMode == "auto"
}

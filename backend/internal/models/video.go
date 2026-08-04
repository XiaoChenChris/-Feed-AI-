package models

import (
	"regexp"
	"time"
)

// Video entity
type Video struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AuthorID     uint      `gorm:"index;not null" json:"author_id"`
	Username     string    `gorm:"type:varchar(255);not null" json:"username"`
	Title        string    `gorm:"type:varchar(255);not null" json:"title"`
	Description  string    `gorm:"type:varchar(255);" json:"description,omitempty"`
	PlayURL      string    `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL     string    `gorm:"type:varchar(255);not null" json:"cover_url"`
	CreateTime   time.Time `gorm:"autoCreateTime;index:idx_videos_create_time,sort:desc;index:idx_videos_popularity_time_id,priority:2,sort:desc" json:"create_time"`
	LikesCount   int64     `gorm:"column:likes_count;not null;default:0;index:idx_videos_likes_count_id,priority:1,sort:desc" json:"likes_count"`
	Popularity   int64     `gorm:"column:popularity;not null;default:0;index:idx_videos_popularity_time_id,priority:1,sort:desc" json:"popularity"`
	ReviewStatus     string  `gorm:"type:varchar(20);default:pending;index" json:"review_status"`
	ReviewReason     string  `gorm:"type:text" json:"review_reason,omitempty"`
	ReviewConfidence float64 `gorm:"type:decimal(5,4);default:0" json:"review_confidence,omitempty"`
	ReviewCategories string  `gorm:"type:varchar(255)" json:"review_categories,omitempty"`
	RetryCount       int     `gorm:"default:0" json:"retry_count,omitempty"`
	PlayCount        int64   `gorm:"column:play_count;not null;default:0" json:"play_count"`
	ReportCount      int     `gorm:"column:report_count;not null;default:0" json:"report_count"`
	LastReviewTime   *time.Time `gorm:"column:last_review_time" json:"last_review_time,omitempty"`
	ReviewPriority   int     `gorm:"column:review_priority;default:0" json:"review_priority,omitempty"`
	AgentTrace   string `gorm:"column:agent_trace;type:json" json:"agent_trace,omitempty"`
	AgentRounds  int    `gorm:"column:agent_rounds;default:0" json:"agent_rounds,omitempty"`
	AgentVerdict string `gorm:"column:agent_verdict;type:varchar(30)" json:"agent_verdict,omitempty"`
	Phase0Result string `gorm:"column:phase0_result;type:json" json:"phase0_result,omitempty"`
	Phase1Result string `gorm:"column:phase1_result;type:json" json:"phase1_result,omitempty"`
}

type PublishVideoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	PlayURL     string `json:"play_url"`
	CoverURL    string `json:"cover_url"`
}

type DeleteVideoRequest struct {
	ID uint `json:"id"`
}

type ListByAuthorIDRequest struct {
	AuthorID uint `json:"author_id"`
}

type GetDetailRequest struct {
	ID uint `json:"id"`
}

type UpdateLikesCountRequest struct {
	ID         uint  `json:"id"`
	LikesCount int64 `json:"likes_count"`
}

type ReSubmitVideoRequest struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// OutboxMsg transactional outbox pattern message
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey"`
	VideoID    uint      `gorm:"index"`
	EventType  string    `gorm:"type:varchar(50)"`
	CreateTime time.Time `gorm:"autoCreateTime"`
	Status     string    `gorm:"type:varchar(50);index"`
}

// Like entity
type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"video_id"`
	AccountID uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

type LikeRequest struct {
	VideoID uint `json:"video_id"`
}

// Comment entity
type Comment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"index" json:"username"`
	VideoID      uint      `gorm:"index" json:"video_id"`
	AuthorID     uint      `gorm:"index" json:"author_id"`
	Content      string    `gorm:"type:text" json:"content"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	ReviewStatus string    `gorm:"type:varchar(20);default:approved;index" json:"review_status"`
	ReviewReason string    `gorm:"type:text" json:"review_reason,omitempty"`
}

type PublishCommentRequest struct {
	VideoID uint   `json:"video_id"`
	Content string `json:"content"`
}

type DeleteCommentRequest struct {
	CommentID uint `json:"comment_id"`
}

type GetAllCommentsRequest struct {
	VideoID uint `json:"video_id"`
}

// Tag entity
type Tag struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex;type:varchar(100);not null" json:"name"`
}

type VideoTag struct {
	ID      uint `gorm:"primaryKey"`
	VideoID uint `gorm:"index;not null"`
	TagID   uint `gorm:"index;not null"`
}

var tagRegex = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

// ExtractTags extracts hashtags from text
func ExtractTags(text string) []string {
	matches := tagRegex.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var tags []string
	for _, m := range matches {
		tag := m[1]
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

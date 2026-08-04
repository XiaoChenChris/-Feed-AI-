package controllers

import (
	"net/http"
	"strconv"

	"feedsystem_ai_go/internal/models"
	"feedsystem_ai_go/internal/services"
	review "feedsystem_ai_go/internal/services/review"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ReviewHandler struct {
	db           *gorm.DB
	service      *review.ReviewService
	videoService *services.VideoService
}

func NewReviewHandler(db *gorm.DB, svc *review.ReviewService, videoService *services.VideoService) *ReviewHandler {
	return &ReviewHandler{db: db, service: svc, videoService: videoService}
}

// GetPendingVideos GET /review/pending
func (h *ReviewHandler) GetPendingVideos(c *gin.Context) {
	var videos []models.Video
	h.db.Where("review_status = ?", "manual_review").
		Order("review_priority DESC, create_time ASC").
		Limit(100).
		Find(&videos)
	if videos == nil {
		videos = []models.Video{}
	}
	c.JSON(http.StatusOK, videos)
}

// ApproveVideo POST /review/video/:id/approve
func (h *ReviewHandler) ApproveVideo(c *gin.Context) {
	h.manualReview(c, "approved")
}

// RejectVideo POST /review/video/:id/reject
func (h *ReviewHandler) RejectVideo(c *gin.Context) {
	h.manualReview(c, "rejected")
}

func (h *ReviewHandler) manualReview(c *gin.Context, status string) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	var v models.Video
	if err := h.db.First(&v, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "video not found"})
		return
	}

	oldStatus := v.ReviewStatus
	v.ReviewStatus = status
	if req.Reason != "" {
		v.ReviewReason = "manual review: " + req.Reason
	} else {
		v.ReviewReason = "manual review"
	}
	v.RetryCount = 0
	h.db.Save(&v)

	if status == "approved" && oldStatus != "approved" {
		msg := models.OutboxMsg{
			VideoID:    v.ID,
			EventType:  "video_published",
			Status:     "pending",
			CreateTime: v.CreateTime,
		}
		h.db.Create(&msg)
	}

	c.JSON(http.StatusOK, gin.H{"message": "review done", "status": status})
}

// GetVideoReviewStatus GET /review/status/:videoId
func (h *ReviewHandler) GetVideoReviewStatus(c *gin.Context) {
	idStr := c.Param("videoId")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ID"})
		return
	}

	var v models.Video
	if err := h.db.First(&v, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "video not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                v.ID,
		"review_status":     v.ReviewStatus,
		"review_reason":     v.ReviewReason,
		"review_confidence": v.ReviewConfidence,
	})
}

// ReSubmitVideo POST /review/resubmit
func (h *ReviewHandler) ReSubmitVideo(c *gin.Context) {
	var req models.ReSubmitVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var v models.Video
	if err := h.db.First(&v, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"error": "video not found"})
		return
	}

	if v.ReviewStatus != "rejected" && v.ReviewStatus != "manual_review" {
		c.JSON(400, gin.H{"error": "can only resubmit rejected or manual_review videos"})
		return
	}

	v.Title = req.Title
	v.Description = req.Description
	v.ReviewStatus = "pending"
	v.ReviewReason = ""
	v.ReviewConfidence = 0
	v.RetryCount = 0
	h.db.Save(&v)

	if h.service.IsEnabled() && h.videoService != nil {
		go h.videoService.ReviewAndPublishVideo(&v)
	}

	c.JSON(http.StatusOK, gin.H{"message": "resubmitted for review"})
}

// GetReviewConfig GET /review/config
func (h *ReviewHandler) GetReviewConfig(c *gin.Context) {
	cfg := h.service.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"enabled":                 cfg.Enabled,
		"text_model":              cfg.TextModel,
		"vision_model":            cfg.VisionModel,
		"sample_frames":           cfg.SampleFrames,
		"frame_review_mode":       cfg.FrameReviewMode,
		"confidence_threshold":    cfg.ConfidenceThreshold,
		"manual_review_threshold": cfg.ManualReviewThreshold,
		"max_retries":             cfg.MaxRetries,
	})
}

// UpdateReviewConfig POST /review/config
func (h *ReviewHandler) UpdateReviewConfig(c *gin.Context) {
	var req struct {
		Enabled               *bool    `json:"enabled"`
		TextModel             *string  `json:"text_model"`
		VisionModel           *string  `json:"vision_model"`
		SampleFrames          *int     `json:"sample_frames"`
		FrameReviewMode       *string  `json:"frame_review_mode"`
		ConfidenceThreshold   *float64 `json:"confidence_threshold"`
		ManualReviewThreshold *float64 `json:"manual_review_threshold"`
		MaxRetries            *int     `json:"max_retries"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	cfg := h.service.GetConfig()
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.TextModel != nil {
		cfg.TextModel = *req.TextModel
	}
	if req.VisionModel != nil {
		cfg.VisionModel = *req.VisionModel
	}
	if req.SampleFrames != nil {
		cfg.SampleFrames = *req.SampleFrames
	}
	if req.FrameReviewMode != nil {
		cfg.FrameReviewMode = *req.FrameReviewMode
	}
	if req.ConfidenceThreshold != nil {
		cfg.ConfidenceThreshold = *req.ConfidenceThreshold
	}
	if req.ManualReviewThreshold != nil {
		cfg.ManualReviewThreshold = *req.ManualReviewThreshold
	}
	if req.MaxRetries != nil {
		cfg.MaxRetries = *req.MaxRetries
	}
	h.service.UpdateConfig(cfg)

	c.JSON(http.StatusOK, gin.H{"message": "review config updated"})
}

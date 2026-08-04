package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"feedsystem_ai_go/pkg/config"
	"feedsystem_ai_go/internal/models"
	"feedsystem_ai_go/pkg/ratelimit"
	"feedsystem_ai_go/pkg/retry"
	review "feedsystem_ai_go/internal/services/review"

	"feedsystem_ai_go/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AIHandler struct {
	db            *gorm.DB
	aiService     *services.AIService
	redis         *redis.Client
	reviewService *review.ReviewService
}

func NewAIHandler(db *gorm.DB, aiService *services.AIService, rdb *redis.Client) *AIHandler {
	return &AIHandler{db: db, aiService: aiService, redis: rdb}
}

func (h *AIHandler) SetReviewService(rs *review.ReviewService) {
	h.reviewService = rs
}

// TriggerAnalysis POST /ai/analyze
func (h *AIHandler) TriggerAnalysis(c *gin.Context) {
	var req struct {
		MediaID uint `json:"media_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请提供 media_id"})
		return
	}

	limiter := ratelimit.NewTokenBucket(h.redis, "limit:ai:global", 10, 1*time.Minute)
	if !limiter.Allow() {
		c.JSON(429, gin.H{"error": "系统繁忙(限流中)，请1分钟后再试"})
		return
	}

	var media models.MediaFile
	if err := h.db.First(&media, req.MediaID).Error; err != nil {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}

	if media.Status == "PROCESSING" {
		c.JSON(200, gin.H{"message": "任务已在后台运行，无需重复提交"})
		return
	}

	media.Status = "PROCESSING"
	h.db.Save(&media)

	go func() {
		retry.WithBackoff(func() error {
			result, err := h.aiService.AnalyzeVideo(media.FilePath)
			if err != nil {
				log.Printf("❌ [AI Worker] 分析失败 (mediaID=%d): %v", media.ID, err)
				media.Status = "FAILED"
				media.AiSummary = fmt.Sprintf("❌ 分析失败: %v", err)
				h.db.Save(&media)
				return err
			}

			media.TranscriptText = result.Transcript
			media.AiSummary = result.Summary
			media.Status = "COMPLETED"
			h.db.Save(&media)

			if h.reviewService != nil && h.reviewService.IsEnabled() {
				go func() {
					reviewResult, err := h.reviewService.ReviewText(result.Summary, "")
					if err != nil {
						log.Printf("[Review] AI summary review failed mediaID=%d: %v", media.ID, err)
						return
					}
					if h.reviewService.Classify(reviewResult) == "rejected" {
						h.db.Model(&media).Updates(map[string]interface{}{
							"ai_summary": "[此内容因违规被隐藏]",
						})
						log.Printf("[Review] AI summary hidden mediaID=%d confidence=%.2f", media.ID, reviewResult.Confidence)
					}
				}()
			}

			if h.reviewService != nil && h.reviewService.IsEnabled() {
				go func() {
					reviewResult, err := h.reviewService.ReviewText(result.Transcript, "")
					if err != nil {
						log.Printf("[音频审核] media_id=%d 审核失败: %v", media.ID, err)
						return
					}
					status := h.reviewService.Classify(reviewResult)
					if status == "rejected" && reviewResult.Confidence >= h.reviewService.GetConfig().ConfidenceThreshold {
						log.Printf("[音频审核] media_id=%d 音频违规: %s (confidence=%.2f)", media.ID, reviewResult.Reason, reviewResult.Confidence)
					}
				}()
			}

			if h.redis != nil {
				userIDStr := "anon"
				if media.UserID != nil {
					userIDStr = strconv.FormatUint(uint64(*media.UserID), 10)
				}
				h.redis.Del(c, "media:list:user:"+userIDStr)
			}

			log.Printf("✅ [AI Worker] 分析完成 (mediaID=%d)", media.ID)
			return nil
		}, 3, 2*time.Second)
	}()

	c.JSON(200, gin.H{"message": "✅ 任务已提交，正在后台分析..."})
}

// TranscribeOnly POST /ai/transcribe
func (h *AIHandler) TranscribeOnly(c *gin.Context) {
	var req struct {
		MediaID uint `json:"media_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请提供 media_id"})
		return
	}

	var media models.MediaFile
	if err := h.db.First(&media, req.MediaID).Error; err != nil {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}

	go func() {
		retry.WithBackoff(func() error {
			tmpDir := ""
			mp3Path := tmpDir + fmt.Sprintf("transcribe_%d.mp3", time.Now().UnixNano())

			if err := h.aiService.ExtractAudio(media.FilePath, mp3Path); err != nil {
				log.Printf("❌ 音频提取失败: %v", err)
				return err
			}

			text, err := h.aiService.TranscribeAudio(mp3Path)
			if err != nil {
				log.Printf("❌ 转写失败: %v", err)
				return err
			}

			media.TranscriptText = text
			h.db.Save(&media)
			log.Printf("✅ 文字提取完成 (mediaID=%d)", media.ID)
			return nil
		}, 3, 2*time.Second)
	}()

	c.JSON(200, gin.H{"message": "✅ 提取任务已后台运行"})
}

// DownloadAudio GET /ai/audio/:id
func (h *AIHandler) DownloadAudio(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的 ID"})
		return
	}

	var media models.MediaFile
	if err := h.db.First(&media, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}

	tmpPath := fmt.Sprintf("%s%caudio_%d.mp3", c.GetString("upload_dir"), '\\', time.Now().UnixNano())
	if err := h.aiService.ExtractAudio(media.FilePath, tmpPath); err != nil {
		c.JSON(500, gin.H{"error": "音频提取失败: " + err.Error()})
		return
	}

	filename := "audio.mp3"
	if media.Filename != "" {
		idx := strings.LastIndex(media.Filename, ".")
		if idx >= 0 {
			filename = media.Filename[:idx] + ".mp3"
		}
	}

	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.File(tmpPath)
}

// GetAnalysisStatus GET /ai/status/:id
func (h *AIHandler) GetAnalysisStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的 ID"})
		return
	}

	var media models.MediaFile
	if err := h.db.First(&media, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              media.ID,
		"status":          media.Status,
		"ai_summary":      media.AiSummary,
		"transcript_text": media.TranscriptText,
	})
}

// SummarizeText POST /ai/summarize
func (h *AIHandler) SummarizeText(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请提供文本内容"})
		return
	}

	limiter := ratelimit.NewTokenBucket(h.redis, "limit:ai:global", 10, 1*time.Minute)
	if !limiter.Allow() {
		c.JSON(429, gin.H{"error": "系统繁忙(限流中)"})
		return
	}

	summary, err := h.aiService.Summarize(req.Text)
	if err != nil {
		c.JSON(500, gin.H{"error": "AI 总结失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// GetConfig GET /ai/config
func (h *AIHandler) GetConfig(c *gin.Context) {
	cfg := h.aiService.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"api_key":   cfg.APIKey,
		"base_url":  cfg.BaseURL,
		"model":     cfg.Model,
		"asr_model": cfg.ASRModel,
	})
}

// UpdateConfig POST /ai/config
func (h *AIHandler) UpdateConfig(c *gin.Context) {
	var req config.AIConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	h.aiService.UpdateConfig(req)
	c.JSON(http.StatusOK, gin.H{"message": "AI 配置已更新", "config": gin.H{
		"base_url":  req.BaseURL,
		"model":     req.Model,
		"asr_model": req.ASRModel,
		"api_key":   "已保存 (不会回显)",
	}})
}

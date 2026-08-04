package controllers

import (
	"io"
	"net/http"
	"strconv"

	"feedsystem_ai_go/internal/models"
	"feedsystem_ai_go/internal/services"
	"feedsystem_ai_go/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type MediaHandler struct {
	service *services.MediaService
	rdb     *redis.Client
}

func NewMediaHandler(svc *services.MediaService, rdb *redis.Client) *MediaHandler {
	return &MediaHandler{service: svc, rdb: rdb}
}

// InitUpload POST /media/init-upload
func (h *MediaHandler) InitUpload(c *gin.Context) {
	uploadID := h.service.InitChunkedUpload()
	c.JSON(http.StatusOK, gin.H{"upload_id": uploadID})
}

// Upload POST /media/upload
func (h *MediaHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "请选择文件"})
		return
	}

	var userID *uint
	if uidStr := c.PostForm("user_id"); uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 64)
		if err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	record, err := h.service.UploadFile(file, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "上传失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "上传成功", "id": record.ID, "url": record.FilePath})
}

// UploadChunk POST /media/upload-chunk
func (h *MediaHandler) UploadChunk(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	if uploadID == "" {
		c.JSON(400, gin.H{"error": "请提供 upload_id"})
		return
	}

	chunkIndexStr := c.PostForm("chunk_index")
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的分片索引"})
		return
	}

	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		c.JSON(400, gin.H{"error": "请选择分片文件"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取分片失败"})
		return
	}

	if err := h.service.UploadChunk(uploadID, chunkIndex, data); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分片上传成功", "chunk_index": chunkIndex})
}

// CompleteChunkUpload POST /media/complete-upload
func (h *MediaHandler) CompleteChunkUpload(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	filename := c.PostForm("filename")
	md5Hash := c.PostForm("md5")

	if uploadID == "" || filename == "" {
		c.JSON(400, gin.H{"error": "请提供 upload_id 和 filename"})
		return
	}

	var userID *uint
	if uidStr := c.PostForm("user_id"); uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 64)
		if err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	var chunks [][]byte
	if h.rdb != nil {
		chunksKey := "upload:chunked:" + uploadID + ":chunks"
		fields, _ := h.rdb.HGetAll(c, chunksKey).Result()
		totalChunks := len(fields)
		_ = totalChunks
	}

	record, err := h.service.CompleteUpload(uploadID, filename, chunks, userID, md5Hash)
	if err != nil {
		c.JSON(500, gin.H{"error": "合并失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "上传完成", "id": record.ID, "url": record.FilePath})
}

// List GET /media/list
func (h *MediaHandler) List(c *gin.Context) {
	var userID *uint
	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 64)
		if err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	records, err := h.service.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	if records == nil {
		records = []models.MediaFileRecord{}
	}
	c.JSON(http.StatusOK, records)
}

// Delete DELETE /media/delete
func (h *MediaHandler) Delete(c *gin.Context) {
	idStr := c.Query("id")
	userIDStr := c.Query("user_id")

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的 ID"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的 user_id"})
		return
	}

	if err := h.service.DeleteMedia(uint(id), uint(userID)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// CheckDuplicate POST /media/check-duplicate
func (h *MediaHandler) CheckDuplicate(c *gin.Context) {
	var req struct {
		MD5 string `json:"md5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MD5 == "" {
		c.JSON(400, gin.H{"error": "请提供 MD5 哈希"})
		return
	}

	dedupLock := storage.NewDistributedLock(h.rdb, "dedup:"+req.MD5)
	acquired, _ := dedupLock.TryLock(0, 5*30)

	if acquired {
		dedupLock.Unlock()
		c.JSON(http.StatusOK, gin.H{"duplicate": false})
	} else {
		c.JSON(http.StatusOK, gin.H{"duplicate": true, "message": "检测到重复文件"})
	}
}

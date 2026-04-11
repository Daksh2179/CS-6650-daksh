package handlers

import (
	"fmt"
	"io"
	"net/http"

	"final-mastery/store"
	"final-mastery/worker"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PhotoHandler struct {
	dynamo *store.DynamoClient
	redis  *store.RedisClient
	s3     *store.S3Client
	bucket string
	pool   *worker.Pool
}

func NewPhotoHandler(dynamo *store.DynamoClient, redis *store.RedisClient, s3 *store.S3Client, bucket string, pool *worker.Pool) *PhotoHandler {
	return &PhotoHandler{dynamo: dynamo, redis: redis, s3: s3, bucket: bucket, pool: pool}
}

func (h *PhotoHandler) UploadPhoto(c *gin.Context) {
	albumID := c.Param("album_id")

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing photo field"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read photo"})
		return
	}

	photoID := uuid.New().String()

	seq, err := h.redis.IncrSeq(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "seq error"})
		return
	}

	if err := h.dynamo.PutPhoto(c.Request.Context(), photoID, albumID, int(seq), "processing", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	ct := header.Header.Get("Content-Type")
	h.pool.Submit(worker.Job{
		PhotoID:     photoID,
		AlbumID:     albumID,
		Data:        data,
		ContentType: ct,
		Bucket:      h.bucket,
	})

	c.JSON(http.StatusAccepted, gin.H{
		"photo_id": photoID,
		"seq":      seq,
		"status":   "processing",
	})
}

func (h *PhotoHandler) GetPhoto(c *gin.Context) {
	photoID := c.Param("photo_id")

	photo, err := h.dynamo.GetPhoto(c.Request.Context(), photoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	if photo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	resp := gin.H{
		"photo_id": photo["photo_id"],
		"album_id": photo["album_id"],
		"seq":      toInt(photo["seq"]),
		"status":   photo["status"],
	}
	if photo["url"] != "" {
		resp["url"] = photo["url"]
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PhotoHandler) DeletePhoto(c *gin.Context) {
	albumID := c.Param("album_id")
	photoID := c.Param("photo_id")

	// delete metadata FIRST so worker cannot write completed after this
	if err := h.dynamo.DeletePhoto(c.Request.Context(), photoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	// delete from S3 best effort
	_ = h.s3.Delete(c.Request.Context(), h.bucket, albumID, photoID)

	c.Status(http.StatusNoContent)
}

func toInt(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}
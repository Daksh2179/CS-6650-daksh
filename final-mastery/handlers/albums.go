package handlers

import (
	"net/http"

	"final-mastery/store"

	"github.com/gin-gonic/gin"
)

type AlbumHandler struct {
	dynamo *store.DynamoClient
}

func NewAlbumHandler(dynamo *store.DynamoClient) *AlbumHandler {
	return &AlbumHandler{dynamo: dynamo}
}

type albumBody struct {
	AlbumID     string `json:"album_id" binding:"required"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Owner       string `json:"owner"` 
}

func (h *AlbumHandler) PutAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	var body albumBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// check if exists to decide 200 vs 201
	existing, err := h.dynamo.GetAlbum(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	if err := h.dynamo.PutAlbum(c.Request.Context(), albumID, body.Title, body.Description, body.Owner); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	status := http.StatusCreated
	if existing != nil {
		status = http.StatusOK
	}

	c.JSON(status, gin.H{
		"album_id":    albumID,
		"title":       body.Title,
		"description": body.Description,
		"owner":       body.Owner,
	})
}

func (h *AlbumHandler) GetAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	album, err := h.dynamo.GetAlbum(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	if album == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"album_id":    album["album_id"],
		"title":       album["title"],
		"description": album["description"],
		"owner":       album["owner"],
	})
}

func (h *AlbumHandler) ListAlbums(c *gin.Context) {
	albums, err := h.dynamo.ListAlbums(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	if albums == nil {
		albums = []map[string]string{}
	}
	c.JSON(http.StatusOK, albums)
}
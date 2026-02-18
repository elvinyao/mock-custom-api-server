package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// listRequests returns paginated request history
func (h *Handler) listRequests(c *gin.Context) {
	if h.recorder == nil {
		c.JSON(http.StatusOK, gin.H{"total": 0, "requests": []interface{}{}})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 50
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	entries := h.recorder.List(limit, offset)
	total := h.recorder.Count()

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"requests": entries,
	})
}

// getRequest returns a single request by ID
func (h *Handler) getRequest(c *gin.Context) {
	if h.recorder == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not enabled"})
		return
	}

	id := c.Param("id")
	entry := h.recorder.Get(id)
	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}

	c.JSON(http.StatusOK, entry)
}

// clearRequests removes all recorded requests
func (h *Handler) clearRequests(c *gin.Context) {
	if h.recorder != nil {
		h.recorder.Clear()
	}
	c.JSON(http.StatusOK, gin.H{"message": "Request history cleared"})
}

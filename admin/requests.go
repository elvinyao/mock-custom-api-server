package admin

import (
	"net/http"
	"strconv"
	"strings"

	"mock-api-server/recorder"

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

	// Filter the whole recorder snapshot before applying pagination. Total is the
	// number of matches from that same snapshot, including older pages.
	method := c.Query("method")
	path := strings.ToLower(c.Query("path"))
	status := strings.ToLower(c.Query("status"))
	entries := make([]*recorder.RecordedRequest, 0)
	for _, entry := range h.recorder.List(0, 0) {
		if method != "" && !strings.EqualFold(method, entry.Method) {
			continue
		}
		if path != "" && !strings.Contains(strings.ToLower(entry.Path), path) {
			continue
		}
		if status != "" {
			code := strconv.Itoa(entry.ResponseStatus)
			class := strconv.Itoa(entry.ResponseStatus / 100)
			if status != code && status != class && status != class+"xx" {
				continue
			}
		}
		entries = append(entries, entry)
	}
	total := len(entries)
	if offset > total {
		offset = total
	}
	entries = entries[offset:]
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}

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

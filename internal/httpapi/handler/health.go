package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	ping    func(ctx context.Context) error
	timeout time.Duration
}

func NewHealthHandler(ping func(ctx context.Context) error, timeout time.Duration) *HealthHandler {
	return &HealthHandler{ping: ping, timeout: timeout}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	if err := h.ping(ctx); err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"patrn.ink/internal/storage"
)

// RefreshLinkHealthHandler checks the current destinations behind a link and stores their status.
func RefreshLinkHealthHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	if err := refreshLinkHealth(link); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to check link destinations"})
		return
	}

	c.JSON(http.StatusOK, link)
}

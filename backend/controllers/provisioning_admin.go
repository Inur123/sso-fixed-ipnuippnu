package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/provisioning"
)

// AdminProvisioningStatus hanya menampilkan metadata operasional. Payload
// identitas dan secret target tidak pernah dikembalikan melalui API.
func AdminProvisioningStatus(c *gin.Context) {
	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var counts []statusCount
	if err := database.DB.Model(&models.ProvisioningOutbox{}).
		Select("status, COUNT(*) AS count").Group("status").Order("status ASC").Scan(&counts).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil status provisioning.")
		return
	}
	var events []models.ProvisioningOutbox
	if err := database.DB.Select("id", "client_id", "event_type", "subject", "status", "attempts", "next_attempt_at", "last_error", "delivered_at", "created_at", "updated_at").
		Where("status IN ?", []string{"pending", "processing", "dead"}).
		Order("created_at DESC").Limit(100).Find(&events).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil antrean provisioning.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"counts": counts, "events": events})
}

func AdminRetryProvisioningEvent(c *gin.Context) {
	eventID := strings.TrimSpace(c.Param("id"))
	result := database.DB.Model(&models.ProvisioningOutbox{}).
		Where("id = ? AND status = ?", eventID, "dead").
		Updates(map[string]any{"status": "pending", "attempts": 0, "next_attempt_at": time.Now().UTC(), "locked_until": nil, "last_error": ""})
	if result.Error != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menjadwalkan ulang event provisioning.")
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "Event provisioning gagal tidak ditemukan.")
		return
	}
	provisioning.Notify()
	auditFromContext(c, AuditOAuthClientAssignmentUpdate, "provisioning_event", eventID, "Event provisioning gagal dijadwalkan ulang.")
	c.JSON(http.StatusOK, gin.H{"message": "Event provisioning dijadwalkan ulang."})
}

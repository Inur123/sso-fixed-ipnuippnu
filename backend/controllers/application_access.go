package controllers

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"sso-backend/models"
)

var errApplicationAccessDenied = errors.New("user is not assigned to this application")

// applicationAccess hanya memutuskan boleh/tidaknya pengguna masuk. Role dan
// permission bisnis dikelola oleh aplikasi tujuan setelah login SSO berhasil.
func applicationAccess(db *gorm.DB, client models.OAuthClient, userID string, now time.Time) (models.OAuthClientAssignment, error) {
	if client.Status != models.ClientStatusActive {
		return models.OAuthClientAssignment{}, errApplicationAccessDenied
	}
	var assignment models.OAuthClientAssignment
	err := db.Where("client_id = ? AND user_id = ? AND is_active = ?", client.ID, userID, true).First(&assignment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if client.AccessPolicy == models.AccessPolicyAllActiveUsers {
				return models.OAuthClientAssignment{}, nil
			}
			return models.OAuthClientAssignment{}, errApplicationAccessDenied
		}
		return models.OAuthClientAssignment{}, err
	}
	return assignment, nil
}

func normalizeAccessPolicy(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return models.AccessPolicyAssignedOnly, nil
	}
	if value != models.AccessPolicyAssignedOnly && value != models.AccessPolicyAllActiveUsers {
		return "", &validationError{"access_policy harus assigned_only atau all_active_users."}
	}
	return value, nil
}

// revokeClientUserGrant menghapus code yang belum ditukar dan mencabut seluruh
// token client-user. Dipakai pada perubahan policy dan assignment agar revokasi
// efektif segera, bukan menunggu access token kedaluwarsa.
func revokeClientUserGrant(tx *gorm.DB, clientID, userID string, now time.Time) error {
	if err := tx.Model(&models.OAuthToken{}).
		Where("client_id = ? AND user_id = ? AND revoked_at IS NULL", clientID, userID).
		Update("revoked_at", now).Error; err != nil {
		return err
	}
	return tx.Where("client_id = ? AND user_id = ?", clientID, userID).Delete(&models.OAuthAuthCode{}).Error
}

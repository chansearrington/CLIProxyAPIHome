package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"gorm.io/gorm"
)

const oauthReconnectContextKey = "home.oauth.reconnect"

func reconnectSupported(auth *coreauth.Auth) bool {
	return isOAuthAuth(auth) && (auth.Provider == "claude" || auth.Provider == "codex") &&
		strings.TrimSpace(stringFromAny(auth.Metadata["email"])) != "" &&
		(auth.Provider != "codex" || strings.TrimSpace(stringFromAny(auth.Metadata["account_id"])) != "")
}

// prepareOAuthReconnect binds the login to an existing account before issuing an authorization URL.
func (h *Handler) prepareOAuthReconnect(c *gin.Context, provider string) bool {
	id := strings.TrimSpace(c.Query("credential_id"))
	if id == "" {
		return true
	}
	ctx, cancel := h.requestContext(c)
	defer cancel()
	auth, _, errGet := h.repo.GetAuth(ctx, id)
	if errors.Is(errGet, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "reconnect credential not found"})
		return false
	}
	if errGet != nil {
		respondError(c, http.StatusInternalServerError, "reconnect_lookup_failed", errGet)
		return false
	}
	if !reconnectSupported(auth) || auth.Provider != provider {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reconnect requires a matching Claude or Codex OAuth credential"})
		return false
	}
	c.Set(oauthReconnectContextKey, map[string]any{
		"reconnect_credential_id": auth.ID,
		"reconnect_email":         stringFromAny(auth.Metadata["email"]),
		"reconnect_account_id":    stringFromAny(auth.Metadata["account_id"]),
	})
	return true
}

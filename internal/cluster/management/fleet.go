package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
)

func (h *Handler) GetFleet(c *gin.Context) {
	ctx, cancel := h.requestContext(c)
	defer cancel()
	state, errState := fleet.New(h.repo).Status(ctx)
	if errState != nil {
		respondError(c, http.StatusInternalServerError, "fleet_state_failed", errState)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *Handler) PutFleetPolicy(c *gin.Context) {
	var policy fleet.Policy
	if errBind := c.ShouldBindJSON(&policy); errBind != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_fleet_policy"})
		return
	}
	if errPolicy := fleet.ValidatePolicy(policy); errPolicy != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_fleet_policy", "message": errPolicy.Error()})
		return
	}
	ctx, cancel := h.requestContext(c)
	defer cancel()
	if errSet := fleet.New(h.repo).SetPolicy(ctx, policy); errSet != nil {
		respondError(c, http.StatusConflict, "fleet_policy_failed", errSet)
		return
	}
	h.GetFleet(c)
}

func (h *Handler) PostponeFleet(c *gin.Context) {
	var request struct {
		TargetVersion string `json:"target_version"`
		NodeID        string `json:"node_id"`
		Force         bool   `json:"force"`
	}
	if errBind := c.ShouldBindJSON(&request); errBind != nil || request.TargetVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_fleet_postponement"})
		return
	}
	ctx, cancel := h.requestContext(c)
	defer cancel()
	if errPostpone := fleet.New(h.repo).Postpone(ctx, request.TargetVersion, request.NodeID, request.Force); errPostpone != nil {
		respondError(c, http.StatusConflict, "fleet_postpone_failed", errPostpone)
		return
	}
	h.GetFleet(c)
}

func (h *Handler) ReviewFleetRelease(c *gin.Context) {
	action := c.Param("action")
	if action != "approve" && action != "hold" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_release_action"})
		return
	}
	ctx, cancel := h.requestContext(c)
	defer cancel()
	if errReview := fleet.New(h.repo).Review(ctx, c.Param("version"), action == "approve"); errReview != nil {
		respondError(c, http.StatusConflict, "fleet_review_failed", errReview)
		return
	}
	h.GetFleet(c)
}

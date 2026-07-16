package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) GetAccountAutoProbeSettings(c *gin.Context) {
	if h.accountAutoProbe == nil {
		response.ErrorFrom(c, service.ErrAccountAutoProbeUnavailable)
		return
	}
	settings, err := h.accountAutoProbe.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AccountHandler) UpdateAccountAutoProbeSettings(c *gin.Context) {
	if h.accountAutoProbe == nil {
		response.ErrorFrom(c, service.ErrAccountAutoProbeUnavailable)
		return
	}
	var req service.AccountAutoProbeSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.accountAutoProbe.UpdateSettings(c.Request.Context(), &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings, err := h.accountAutoProbe.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
